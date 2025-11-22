package kiro

import (
	"errors"
	"fmt"
)

// Authentication error types
var (
	// ErrDeviceCodeExpired is returned when the device code has expired.
	ErrDeviceCodeExpired = errors.New("device code expired")

	// ErrAuthorizationPending is returned when authorization is still pending.
	ErrAuthorizationPending = errors.New("authorization pending")

	// ErrSlowDown is returned when the polling rate is too high.
	ErrSlowDown = errors.New("slow down - reduce polling rate")

	// ErrAccessDenied is returned when the user denies authorization.
	ErrAccessDenied = errors.New("access denied by user")

	// ErrInvalidToken is returned when the token is invalid or malformed.
	ErrInvalidToken = errors.New("invalid token")

	// ErrTokenExpired is returned when the token has expired.
	ErrTokenExpired = errors.New("token expired")

	// ErrRefreshFailed is returned when token refresh fails.
	ErrRefreshFailed = errors.New("failed to refresh token")

	// ErrInvalidRefreshToken is returned when the refresh token is invalid.
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
)

// AuthError wraps authentication errors with additional context.
type AuthError struct {
	Op  string // Operation that failed
	Err error  // Underlying error
	Msg string // Additional context message
}

// Error implements the error interface.
func (e *AuthError) Error() string {
	if e.Msg != "" {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Msg, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error.
func (e *AuthError) Unwrap() error {
	return e.Err
}

// NewAuthError creates a new AuthError.
func NewAuthError(op string, err error, msg string) *AuthError {
	return &AuthError{
		Op:  op,
		Err: err,
		Msg: msg,
	}
}

// IsRetryable returns true if the error indicates a retry should be attempted.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAuthorizationPending) || errors.Is(err, ErrSlowDown)
}
