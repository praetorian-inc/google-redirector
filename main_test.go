package main

import (
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

func TestProxy_ServeHTTP(t *testing.T) {
	// Create a test server to act as the backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Backend response"))
	}))
	defer backend.Close()

	// Parse backend URL
	backendURL, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	// Test proxy functionality
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	body := w.Body.String()
	if body != "Backend response" {
		t.Errorf("Expected 'Backend response', got %s", body)
	}
}

func TestProxy_Methods(t *testing.T) {
	// Create a test server to act as the backend
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(r.Method))
	}))
	defer backend.Close()

	// Parse backend URL
	backendURL, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	methods := []string{"GET", "POST", "PUT", "DELETE"}
	
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		w := httptest.NewRecorder()
		proxy.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Method %s: expected status 200, got %d", method, w.Code)
		}

		if w.Body.String() != method {
			t.Errorf("Method %s: expected %s, got %s", method, method, w.Body.String())
		}
	}
}