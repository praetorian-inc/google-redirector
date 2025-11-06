package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	backendURL := getEnv("BACKEND_URL", "https://your-backend-server.com")

	target, err := url.Parse(backendURL)
	if err != nil {
		log.Fatalf("Failed to parse BACKEND_URL: %v", err)
	}

	// Always skip TLS verification for simplicity
	proxy := httputil.NewSingleHostReverseProxy(target)

	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Simple logging
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		log.Printf("%s %s -> %s", req.Method, req.URL.Path, req.URL.String())
	}

	// Error handler
	proxy.ErrorHandler = func(rw http.ResponseWriter, req *http.Request, err error) {
		log.Printf("Proxy error: %v", err)
		rw.WriteHeader(http.StatusBadGateway)
		rw.Write([]byte("Bad Gateway"))
	}

	// WebSocket and HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Check if this is a WebSocket upgrade request
		if isWebSocketRequest(r) {
			handleWebSocket(w, r, target)
		} else {
			proxy.ServeHTTP(w, r)
		}
	})

	log.Printf("Google redirector starting on port 8080")
	log.Printf("Proxying to: %s", backendURL)
	log.Printf("TLS verification: disabled")
	log.Printf("WebSocket support: enabled")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func isWebSocketRequest(r *http.Request) bool {
	return strings.ToLower(r.Header.Get("Upgrade")) == "websocket" &&
		strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade")
}

func handleWebSocket(w http.ResponseWriter, r *http.Request, target *url.URL) {
	log.Printf("WebSocket upgrade request: %s %s", r.Method, r.URL.Path)

	// Build backend WebSocket URL
	backendURL := &url.URL{
		Scheme:   "ws",
		Host:     target.Host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}
	if target.Scheme == "https" {
		backendURL.Scheme = "wss"
	}

	log.Printf("Connecting to backend WebSocket: %s", backendURL)

	// Connect to backend
	backendConn, backendResp, err := dialBackendWebSocket(backendURL, r)
	if err != nil {
		log.Printf("Backend WebSocket dial failed: %v", err)
		http.Error(w, "Failed to connect to backend", http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	// Hijack client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("Hijacking not supported")
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Hijack failed: %v", err)
		return
	}
	defer clientConn.Close()

	// Send 101 Switching Protocols response to client
	if err := writeSwitchingProtocols(clientConn, r, backendResp); err != nil {
		log.Printf("Failed to send upgrade response: %v", err)
		return
	}

	log.Printf("WebSocket connection established, proxying data...")

	// Bidirectional copy
	var wg sync.WaitGroup
	wg.Add(2)

	go pipe(backendConn, clientConn, "client→backend", &wg)
	go pipe(clientConn, backendConn, "backend→client", &wg)

	wg.Wait()
}

func dialBackendWebSocket(u *url.URL, r *http.Request) (net.Conn, *http.Response, error) {
	// Determine host and port
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	// Dial TCP connection
	conn, err := net.DialTimeout("tcp", host, 10*time.Second)
	if err != nil {
		return nil, nil, err
	}

	// Wrap with TLS if wss
	if u.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         u.Hostname(),
			InsecureSkipVerify: true,
		})
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, nil, err
		}
		conn = tlsConn
	}

	// Build WebSocket upgrade request
	req := &http.Request{
		Method: "GET",
		URL:    u,
		Header: make(http.Header),
		Host:   u.Host,
	}
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")

	// Forward important headers
	req.Header.Set("Sec-WebSocket-Version", r.Header.Get("Sec-WebSocket-Version"))
	req.Header.Set("Sec-WebSocket-Key", r.Header.Get("Sec-WebSocket-Key"))

	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		req.Header.Set("Sec-WebSocket-Protocol", proto)
	}

	if ext := r.Header.Get("Sec-WebSocket-Extensions"); ext != "" {
		req.Header.Set("Sec-WebSocket-Extensions", ext)
	}

	if auth := r.Header.Get("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	// Send upgrade request
	if err := req.Write(conn); err != nil {
		conn.Close()
		return nil, nil, err
	}

	// Read response
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		conn.Close()
		return nil, nil, fmt.Errorf("expected 101, got %d", resp.StatusCode)
	}

	return conn, resp, nil
}

func writeSwitchingProtocols(clientConn net.Conn, clientReq *http.Request, backendResp *http.Response) error {
	accept := backendResp.Header.Get("Sec-WebSocket-Accept")
	if accept == "" {
		return fmt.Errorf("missing Sec-WebSocket-Accept from backend")
	}

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n"

	// Forward protocol if both sides agree
	if proto := clientReq.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		backendProto := backendResp.Header.Get("Sec-WebSocket-Protocol")
		if backendProto != "" && strings.Contains(proto, backendProto) {
			resp += fmt.Sprintf("Sec-WebSocket-Protocol: %s\r\n", backendProto)
		}
	}

	resp += "\r\n"

	_, err := clientConn.Write([]byte(resp))
	return err
}

func pipe(dst, src net.Conn, dir string, wg *sync.WaitGroup) {
	defer wg.Done()

	n, err := io.Copy(dst, src)

	if err != nil && !strings.Contains(err.Error(), "use of closed network connection") {
		log.Printf("pipe %s error: %v (copied %d bytes)", dir, err, n)
	} else {
		log.Printf("pipe %s finished (copied %d bytes)", dir, n)
	}

	// 1. WebSocket close frame
	_ = dst.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = dst.Write([]byte{0x88, 0x02, 0x03, 0xe8})

	// 2. Full TLS shutdown (if applicable)
	if tc, ok := dst.(*tls.Conn); ok {
		_ = tc.Close() // sends + drains close_notify
	} else {
		// 3. For plain TCP: half-close + full close
		if sc, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = sc.CloseWrite()
		}
		_ = dst.Close()
	}
}
