// This file contains HTTP handler tests for the server.
//
// KEY GO TESTING CONCEPTS IN THIS FILE:
//   - net/http/httptest for testing HTTP handlers without a network
//   - httptest.NewRecorder for capturing HTTP responses
//   - http.NewRequest for creating test requests
//   - Testing HTTP status codes and response bodies
//
// LEARNING POINT — httptest Package:
// The net/http/httptest package is one of Go's killer features for testing.
// It lets you test HTTP handlers in-process, without starting a real server
// or making real network calls. This makes tests fast, reliable, and free of
// port conflicts. You create a fake request, pass it to your handler, and
// inspect the recorded response.
package main

import (
	"net/http"

	// net/http/httptest provides utilities for HTTP testing:
	//   - httptest.NewRecorder(): creates a fake ResponseWriter that captures
	//     the response (status code, headers, body) for inspection
	//   - httptest.NewServer(): creates a real HTTP server on a random port
	//     (used in websocket_test.go for integration tests)
	"net/http/httptest"
	"testing"
)

// TestHelloHandler tests the "/" endpoint by sending a GET request and
// verifying the response status code and body.
//
// LEARNING POINT — The httptest.NewRecorder Pattern:
// This is the most common pattern for testing HTTP handlers in Go:
//  1. Create a Server with dependencies (here, a Hub)
//  2. Get the router via SetupRouter (exported for testability)
//  3. Create a request with http.NewRequest
//  4. Create a response recorder with httptest.NewRecorder
//  5. Call mux.ServeHTTP(recorder, request) to execute the handler
//  6. Inspect the recorder's Code (status) and Body (response)
//
// This approach tests the handler through the real router, which also verifies
// that routes are registered correctly. It's an in-process test — no real HTTP
// server is started, no ports are used, and it runs in microseconds.
func TestHelloHandler(t *testing.T) {
	// Create a Server with a real Hub. For handler tests that don't use the
	// hub, you could also pass a nil hub — but using NewHub() is safer and
	// ensures the server is in a valid state.
	server := &Server{hub: NewHub()}
	mux := SetupRouter(server)

	// http.NewRequest creates an *http.Request for testing. It doesn't make
	// a real HTTP call — it just constructs the request object. The third
	// argument (nil) is the request body (not needed for GET requests).
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	// httptest.NewRecorder creates a *httptest.ResponseRecorder, which
	// implements http.ResponseWriter. It captures everything the handler
	// writes: status code (rr.Code), headers (rr.Header()), and body
	// (rr.Body, a *bytes.Buffer).
	rr := httptest.NewRecorder()

	// ServeHTTP dispatches the request to the matching handler in the mux.
	// This is the same method the real HTTP server calls for each request.
	mux.ServeHTTP(rr, req)

	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check the response body is what we expect.
	// rr.Body.String() returns the full response body as a string.
	expected := "Hello, World!"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}
