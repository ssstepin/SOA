package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
)

type ProxyHandler struct {
	paths  map[string]string
	client *http.Client
}

func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{
		paths: map[string]string{
			"/signup": "http://auth:8091/signup",
			"/login":  "http://auth:8091/login",
			"/update": "http://auth:8091/update",
			"/whoami": "http://auth:8091/whoami",
		},
		client: &http.Client{},
	}
}

func (p *ProxyHandler) readRequestBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}

func (p *ProxyHandler) createForwardRequest(r *http.Request, path string, body []byte) (*http.Request, error) {
	return http.NewRequest(r.Method, path, bytes.NewReader(body))
}

func (p *ProxyHandler) forwardRequest(req *http.Request) (*http.Response, error) {
	return p.client.Do(req)
}

func (p *ProxyHandler) readResponseBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (p *ProxyHandler) copyHeadersAndBody(w http.ResponseWriter, resp *http.Response, body []byte) {
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

func (p *ProxyHandler) handleRequest(w http.ResponseWriter, r *http.Request) {
	// Read request body
	rbody, err := p.readRequestBody(r)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read request: %v\n", err)
		http.Error(w, "Failed to read request", http.StatusInternalServerError)
		return
	}

	// Get target path
	targetPath, ok := p.paths[r.URL.Path]
	if !ok {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Create forward request
	req, err := p.createForwardRequest(r, targetPath, rbody)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create request: %v\n", err)
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}
	req.Header = r.Header

	// Forward request
	resp, err := p.forwardRequest(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to forward request: %v\n", err)
		http.Error(w, "Failed to forward request", http.StatusInternalServerError)
		return
	}

	// Read response body
	body, err := p.readResponseBody(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read response: %v\n", err)
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// Copy headers and body to original response
	p.copyHeadersAndBody(w, resp, body)
}

func main() {
	proxy := NewProxyHandler()
	http.HandleFunc("/", proxy.handleRequest)

	port := 8092
	fmt.Printf("Starting server at port %d\n", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
