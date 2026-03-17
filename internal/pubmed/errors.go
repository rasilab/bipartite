package pubmed

import (
	"errors"
	"fmt"
)

// Common errors returned by the PubMed client.
var (
	// ErrNotFound indicates the paper was not found in PubMed.
	ErrNotFound = errors.New("paper not found in PubMed")

	// ErrRateLimited indicates the rate limit has been exceeded.
	ErrRateLimited = errors.New("PubMed rate limit exceeded")

	// ErrNetworkError indicates a network connectivity issue.
	ErrNetworkError = errors.New("network error communicating with PubMed")

	// ErrInvalidResponse indicates an unexpected API response.
	ErrInvalidResponse = errors.New("invalid response from PubMed")
)

// APIError represents an error from the PubMed E-utilities API.
type APIError struct {
	StatusCode int
	Message    string
	RetryAfter int // Seconds to wait before retrying (for rate limits)
}

func (e *APIError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("PubMed API error (status %d): %s (retry after %ds)", e.StatusCode, e.Message, e.RetryAfter)
	}
	return fmt.Sprintf("PubMed API error (status %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error indicates a paper was not found.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}

// IsRateLimited returns true if the error indicates the rate limit was exceeded.
func IsRateLimited(err error) bool {
	if errors.Is(err, ErrRateLimited) {
		return true
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429
	}
	return false
}
