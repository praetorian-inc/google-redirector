package main

import (
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	backendURL := getEnv("BACKEND_URL", "https://your-backend-server.com")
	
	target, err := url.Parse(backendURL)
	if err != nil {
		log.Fatalf("Failed to parse BACKEND_URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	
	// Always skip TLS verification for simplicity
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
	backendURL := target.Scheme + "://" + target.Host + r.URL.Path
	if r.URL.RawQuery != "" {
		backendURL += "?" + r.URL.RawQuery
	}

	// Change scheme to ws/wss
	backendURL = strings.Replace(backendURL, "https://", "wss://", 1)
	backendURL = strings.Replace(backendURL, "http://", "ws://", 1)

	log.Printf("Connecting to backend WebSocket: %s", backendURL)

	// Connect to backend
	backendConn, err := dialBackendWebSocket(backendURL, r)
	if err != nil {
		log.Printf("Failed to connect to backend WebSocket: %v", err)
		http.Error(w, "Failed to connect to backend", http.StatusBadGateway)
		return
	}
	defer backendConn.Close()

	// Hijack the client connection
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Printf("HTTP Hijacking not supported")
		http.Error(w, "HTTP Hijacking not supported", http.StatusInternalServerError)
		return
	}

	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Failed to hijack connection: %v", err)
		return
	}
	defer clientConn.Close()

	// Send 101 Switching Protocols response to client
	response := "HTTP/1.1 101 Switching Protocols\r\n"
	response += "Upgrade: websocket\r\n"
	response += "Connection: Upgrade\r\n"

	// Forward Sec-WebSocket-Accept if present
	if accept := r.Header.Get("Sec-WebSocket-Accept"); accept != "" {
		response += "Sec-WebSocket-Accept: " + accept + "\r\n"
	}

	// Forward Sec-WebSocket-Protocol if present
	if protocol := r.Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		response += "Sec-WebSocket-Protocol: " + protocol + "\r\n"
	}

	response += "\r\n"

	if _, err := clientConn.Write([]byte(response)); err != nil {
		log.Printf("Failed to send upgrade response: %v", err)
		return
	}

	log.Printf("WebSocket connection established, proxying data...")

	// Bidirectional copy
	done := make(chan struct{}, 2)

	// Backend -> Client
	go func() {
		io.Copy(clientConn, backendConn)
		done <- struct{}{}
	}()

	// Client -> Backend
	go func() {
		io.Copy(backendConn, clientConn)
		done <- struct{}{}
	}()

	// Wait for either direction to close
	<-done
	log.Printf("WebSocket connection closed")
}

func dialBackendWebSocket(backendURL string, r *http.Request) (net.Conn, error) {
	// Parse backend URL
	u, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}

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
		return nil, err
	}

	// Wrap with TLS if wss
	if u.Scheme == "wss" {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         u.Hostname(),
			InsecureSkipVerify: true,
		})
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}

	// Build WebSocket upgrade request
	upgradeReq := "GET " + u.Path
	if u.RawQuery != "" {
		upgradeReq += "?" + u.RawQuery
	}
	upgradeReq += " HTTP/1.1\r\n"
	upgradeReq += "Host: " + u.Host + "\r\n"
	upgradeReq += "Upgrade: websocket\r\n"
	upgradeReq += "Connection: Upgrade\r\n"

	// Forward important headers
	if key := r.Header.Get("Sec-WebSocket-Key"); key != "" {
		upgradeReq += "Sec-WebSocket-Key: " + key + "\r\n"
	}
	if version := r.Header.Get("Sec-WebSocket-Version"); version != "" {
		upgradeReq += "Sec-WebSocket-Version: " + version + "\r\n"
	}
	if protocol := r.Header.Get("Sec-WebSocket-Protocol"); protocol != "" {
		upgradeReq += "Sec-WebSocket-Protocol: " + protocol + "\r\n"
	}
	if auth := r.Header.Get("Authorization"); auth != "" {
		upgradeReq += "Authorization: " + auth + "\r\n"
	}

	upgradeReq += "\r\n"

	// Send upgrade request
	if _, err := conn.Write([]byte(upgradeReq)); err != nil {
		conn.Close()
		return nil, err
	}

	// Read upgrade response
	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil {
		conn.Close()
		return nil, err
	}

	response := string(buf[:n])
	if !strings.Contains(response, "101") && !strings.Contains(response, "Switching Protocols") {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
