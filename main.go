package main

import (
	"crypto/tls"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
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

	http.HandleFunc("/", proxy.ServeHTTP)
	
	log.Printf("Google redirector starting on port 8080")
	log.Printf("Proxying to: %s", backendURL)
	log.Printf("TLS verification: disabled")
	
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