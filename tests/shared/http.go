package testutil

import (
	"net/http"
)

// RT is a function type that implements http.RoundTripper
type RT func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper interface
func (f RT) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// Client creates an http.Client with a custom RoundTripper
func Client(rt RT) *http.Client {
	return &http.Client{Transport: rt}
}