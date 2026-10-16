// Package shortener holds the core domain: the URL type, its validation
// rules, short-code generation, and the errors other packages check for.
// It knows nothing about HTTP or Postgres.
package shortener

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Sentinel errors. Callers check them with errors.Is, even when wrapped.
var (
	ErrNotFound   = errors.New("short url not found")
	ErrInvalidURL = errors.New("invalid url")
)

// maxURLLength keeps absurdly long input out of the database.
const maxURLLength = 2048

// URL is one shortened link.
type URL struct {
	ID          int64
	LongURL     string
	ShortCode   string
	AccessCount int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ValidateURL checks that raw is an absolute http(s) URL and returns it trimmed.
// Every error it returns wraps ErrInvalidURL.
func ValidateURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("%w: url is required", ErrInvalidURL)
	}
	if len(raw) > maxURLLength {
		return "", fmt.Errorf("%w: must be at most %d characters", ErrInvalidURL, maxURLLength)
	}

	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%w: could not parse", ErrInvalidURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%w: must start with http:// or https://", ErrInvalidURL)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%w: missing host", ErrInvalidURL)
	}

	return raw, nil
}
