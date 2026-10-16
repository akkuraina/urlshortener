package shortener

import (
	"errors"
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	seen := make(map[string]bool)
	for range 1000 {
		code, err := NewCode()
		if err != nil {
			t.Fatalf("NewCode() error: %v", err)
		}
		if len(code) != CodeLength {
			t.Errorf("len(%q) = %d, want %d", code, len(code), CodeLength)
		}
		for _, c := range code {
			if !strings.ContainsRune(alphabet, c) {
				t.Errorf("code %q contains %q, which is not in the alphabet", code, c)
			}
		}
		if seen[code] {
			t.Errorf("duplicate code %q", code)
		}
		seen[code] = true
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string
		wantErr bool
	}{
		{name: "https", in: "https://example.com/a/b?c=1", want: "https://example.com/a/b?c=1"},
		{name: "http", in: "http://example.com", want: "http://example.com"},
		{name: "trims spaces", in: "  https://example.com  ", want: "https://example.com"},
		{name: "empty", in: "", wantErr: true},
		{name: "only spaces", in: "   ", wantErr: true},
		{name: "no scheme", in: "example.com", wantErr: true},
		{name: "ftp scheme", in: "ftp://example.com", wantErr: true},
		{name: "javascript scheme", in: "javascript:alert(1)", wantErr: true},
		{name: "no host", in: "https://", wantErr: true},
		{name: "too long", in: "https://example.com/" + strings.Repeat("a", maxURLLength), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ValidateURL(tt.in)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidURL) {
					t.Fatalf("ValidateURL(%q) error = %v, want ErrInvalidURL", tt.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateURL(%q) unexpected error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("ValidateURL(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
