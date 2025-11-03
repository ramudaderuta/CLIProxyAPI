package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	authkiro "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
	cliproxyauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// KiroTestFixtures provides common test fixtures for Kiro tests
type KiroTestFixtures struct{}

// NewKiroTestFixtures creates a new instance of KiroTestFixtures
func NewKiroTestFixtures() *KiroTestFixtures {
	return &KiroTestFixtures{}
}

// RoundTripperFunc is a function type that implements http.RoundTripper
type RoundTripperFunc func(*http.Request) (*http.Response, error)

// RoundTrip implements http.RoundTripper interface
func (fn RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// WithRoundTripper adds a round tripper to the context
func (f *KiroTestFixtures) WithRoundTripper(ctx context.Context, fn RoundTripperFunc) context.Context {
	return context.WithValue(ctx, "cliproxy.roundtripper", http.RoundTripper(fn))
}

// NewTestToken creates a test Kiro token
func (f *KiroTestFixtures) NewTestToken() *authkiro.KiroTokenStorage {
	return &authkiro.KiroTokenStorage{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-west-2:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		AuthMethod:   "social",
		Provider:     "Github",
		Type:         "kiro",
	}
}

// NewTestAuth creates a test auth object
func (f *KiroTestFixtures) NewTestAuth(token *authkiro.KiroTokenStorage, attrs map[string]string) *cliproxyauth.Auth {
	if token == nil {
		token = f.NewTestToken()
	}
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &cliproxyauth.Auth{
		ID:         "auth-test",
		Provider:   "kiro",
		Attributes: attrs,
		Metadata:   map[string]any{},
		Runtime:    token,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// OpenAIChatPayload creates an OpenAI chat payload for testing
func (f *KiroTestFixtures) OpenAIChatPayload(t testing.TB, messages []map[string]any, tools []map[string]any) []byte {
	t.Helper()
	payload := map[string]any{
		"model":    "claude-sonnet-4-5",
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

// OpenAIChatPayloadNoHelper creates payload without calling t.Helper() to work with testing.B
func (f *KiroTestFixtures) OpenAIChatPayloadNoHelper(messages []map[string]any, tools []map[string]any) []byte {
	payload := map[string]any{
		"model":    "claude-sonnet-4-5",
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return data
}

// Common test data
var (
	// NativeTokenFilePath is the path to the native Kiro token file (without "type": "kiro")
	NativeTokenFilePath = "/home/build/code/CLIProxyAPI/tmp/kiro-test/kiro-auth-token.json"

	// EnhancedTokenFilePath is the path to the enhanced Kiro token file (with "type": "kiro")
	EnhancedTokenFilePath = "/home/build/.cli-proxy-api/kiro-auth-token.json"
)

// Deprecated: Use KiroTestFixtures instead
type roundTripperFunc func(*http.Request) (*http.Response, error)

// Deprecated: Use KiroTestFixtures instead
func (fn roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

// Deprecated: Use KiroTestFixtures instead
func withRoundTripper(ctx context.Context, fn roundTripperFunc) context.Context {
	return context.WithValue(ctx, "cliproxy.roundtripper", http.RoundTripper(fn))
}

// Deprecated: Use KiroTestFixtures instead
func newTestToken() *authkiro.KiroTokenStorage {
	return &authkiro.KiroTokenStorage{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ProfileArn:   "arn:aws:codewhisperer:us-west-2:123456789012:profile/test",
		ExpiresAt:    time.Now().Add(30 * time.Minute),
		AuthMethod:   "social",
		Provider:     "Github",
		Type:         "kiro",
	}
}

// Deprecated: Use KiroTestFixtures instead
func newTestAuth(token *authkiro.KiroTokenStorage, attrs map[string]string) *cliproxyauth.Auth {
	if token == nil {
		token = newTestToken()
	}
	if attrs == nil {
		attrs = map[string]string{}
	}
	return &cliproxyauth.Auth{
		ID:         "auth-test",
		Provider:   "kiro",
		Attributes: attrs,
		Metadata:   map[string]any{},
		Runtime:    token,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
}

// Deprecated: Use KiroTestFixtures instead
func openAIChatPayload(t testing.TB, messages []map[string]any, tools []map[string]any) []byte {
	t.Helper()
	payload := map[string]any{
		"model":    "claude-sonnet-4-5",
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return data
}

// Deprecated: Use KiroTestFixtures instead
// openAIChatPayloadNoHelper creates payload without calling t.Helper() to work with testing.B
func openAIChatPayloadNoHelper(messages []map[string]any, tools []map[string]any) []byte {
	payload := map[string]any{
		"model":    "claude-sonnet-4-5",
		"messages": messages,
	}
	if len(tools) > 0 {
		payload["tools"] = tools
	}
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return data
}