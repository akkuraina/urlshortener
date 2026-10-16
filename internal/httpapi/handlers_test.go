// These tests use package httpapi_test (not httpapi), so they can only use
// the exported API: New, Routes, Store. That tests the real router and
// middleware end to end, with a fake store instead of Postgres.
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"urlshortener/internal/httpapi"
	"urlshortener/internal/shortener"
)

// fakeStore is an in-memory httpapi.Store. If err is set, every method fails with it.
type fakeStore struct {
	urls   map[string]shortener.URL
	nextID int64
	err    error
}

// Compile-time check that *fakeStore satisfies the interface.
var _ httpapi.Store = (*fakeStore)(nil)

var (
	created = time.Date(2021, 9, 1, 12, 0, 0, 0, time.UTC)
	updated = time.Date(2021, 9, 1, 12, 30, 0, 0, time.UTC)
)

func newFakeStore() *fakeStore {
	return &fakeStore{urls: make(map[string]shortener.URL)}
}

func (s *fakeStore) Create(ctx context.Context, longURL string) (shortener.URL, error) {
	if s.err != nil {
		return shortener.URL{}, s.err
	}
	s.nextID++
	u := shortener.URL{
		ID:        s.nextID,
		LongURL:   longURL,
		ShortCode: fmt.Sprintf("code%03d", s.nextID),
		CreatedAt: created,
		UpdatedAt: created,
	}
	s.urls[u.ShortCode] = u
	return u, nil
}

func (s *fakeStore) Get(ctx context.Context, code string) (shortener.URL, error) {
	if s.err != nil {
		return shortener.URL{}, s.err
	}
	u, ok := s.urls[code]
	if !ok {
		return shortener.URL{}, shortener.ErrNotFound
	}
	return u, nil
}

func (s *fakeStore) Visit(ctx context.Context, code string) (shortener.URL, error) {
	u, err := s.Get(ctx, code)
	if err != nil {
		return shortener.URL{}, err
	}
	// Map values aren't addressable: modify a copy, then store it back.
	u.AccessCount++
	s.urls[code] = u
	return u, nil
}

func (s *fakeStore) Update(ctx context.Context, code, longURL string) (shortener.URL, error) {
	u, err := s.Get(ctx, code)
	if err != nil {
		return shortener.URL{}, err
	}
	u.LongURL = longURL
	u.UpdatedAt = updated
	s.urls[code] = u
	return u, nil
}

func (s *fakeStore) Delete(ctx context.Context, code string) error {
	if _, err := s.Get(ctx, code); err != nil {
		return err
	}
	delete(s.urls, code)
	return nil
}

// do sends one request through the full handler and records the response.
func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, r))
	return rec
}

// decode parses the response body as JSON into a T.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decoding body %q: %v", rec.Body.String(), err)
	}
	return v
}

// newAPI returns a handler backed by a store holding one URL, "code001".
func newAPI(t *testing.T) (http.Handler, *fakeStore) {
	t.Helper()
	store := newFakeStore()
	if _, err := store.Create(t.Context(), "https://www.example.com/some/long/url"); err != nil {
		t.Fatal(err)
	}
	return httpapi.New(store).Routes(), store
}

func TestCreate(t *testing.T) {
	h, _ := newAPI(t)

	rec := do(t, h, "POST", "/shorten", `{"url":"https://example.com/new"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Location"); got != "/shorten/code002" {
		t.Errorf("Location = %q, want /shorten/code002", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	// Decode into a map to check the exact JSON shape, including types.
	body := decode[map[string]any](t, rec)
	want := map[string]any{
		"id":        "2", // a string, per the spec
		"url":       "https://example.com/new",
		"shortCode": "code002",
		"createdAt": "2021-09-01T12:00:00Z",
		"updatedAt": "2021-09-01T12:00:00Z",
	}
	if len(body) != len(want) {
		t.Errorf("body has keys %v, want exactly %v", body, want)
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%q] = %#v, want %#v", k, body[k], v)
		}
	}
}

func TestGetCountsAccessButStatsDoesNot(t *testing.T) {
	h, _ := newAPI(t)

	stats := decode[map[string]any](t, do(t, h, "GET", "/shorten/code001/stats", ""))
	// JSON numbers decode into float64 when the target is any.
	if stats["accessCount"] != 0.0 {
		t.Errorf("initial accessCount = %#v, want 0 (and present)", stats["accessCount"])
	}

	for range 2 {
		if rec := do(t, h, "GET", "/shorten/code001", ""); rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want 200", rec.Code)
		}
	}
	do(t, h, "GET", "/code001", "") // a redirect counts too
	do(t, h, "GET", "/shorten/code001/stats", "")

	stats = decode[map[string]any](t, do(t, h, "GET", "/shorten/code001/stats", ""))
	if stats["accessCount"] != 3.0 {
		t.Errorf("accessCount = %v, want 3", stats["accessCount"])
	}
}

func TestUpdate(t *testing.T) {
	h, _ := newAPI(t)

	rec := do(t, h, "PUT", "/shorten/code001", `{"url":"https://example.com/updated"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body)
	}
	body := decode[map[string]any](t, rec)
	if body["url"] != "https://example.com/updated" || body["updatedAt"] != "2021-09-01T12:30:00Z" {
		t.Errorf("body = %v", body)
	}
}

func TestDelete(t *testing.T) {
	h, store := newAPI(t)

	rec := do(t, h, "DELETE", "/shorten/code001", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("204 response has body %q", rec.Body)
	}
	if _, ok := store.urls["code001"]; ok {
		t.Error("URL still in store after DELETE")
	}
}

func TestRedirect(t *testing.T) {
	h, _ := newAPI(t)

	rec := do(t, h, "GET", "/code001", "")

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302", rec.Code)
	}
	if got := rec.Header().Get("Location"); got != "https://www.example.com/some/long/url" {
		t.Errorf("Location = %q", got)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantError  string
	}{
		{"create empty body", "POST", "/shorten", "", 400, "request body is empty"},
		{"create bad json", "POST", "/shorten", `{"url":`, 400, "request body is not valid JSON"},
		{"create unknown field", "POST", "/shorten", `{"uri":"https://x.com"}`, 400, `unknown field "uri"`},
		{"create invalid url", "POST", "/shorten", `{"url":"example.com"}`, 400, "invalid url: must start with http:// or https://"},
		{"get missing", "GET", "/shorten/nope", "", 404, "short url not found"},
		{"update missing", "PUT", "/shorten/nope", `{"url":"https://x.com"}`, 404, "short url not found"},
		{"update invalid url", "PUT", "/shorten/code001", `{"url":"ftp://x.com"}`, 400, "invalid url: must start with http:// or https://"},
		{"delete missing", "DELETE", "/shorten/nope", "", 404, "short url not found"},
		{"stats missing", "GET", "/shorten/nope/stats", "", 404, "short url not found"},
		{"redirect missing", "GET", "/nope", "", 404, "short url not found"},
		{"wrong method", "PATCH", "/shorten/code001", "", 405, "Method Not Allowed"},
		{"no such route", "GET", "/a/b/c", "", 404, "Not Found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newAPI(t)

			rec := do(t, h, tt.method, tt.path, tt.body)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := decode[map[string]string](t, rec)["error"]; got != tt.wantError {
				t.Errorf("error = %q, want %q", got, tt.wantError)
			}
		})
	}
}

func TestStoreFailureDoesNotLeak(t *testing.T) {
	h, store := newAPI(t)
	store.err = errors.New("connecting to 10.0.0.5: password authentication failed")

	rec := do(t, h, "GET", "/shorten/code001", "")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if got := decode[map[string]string](t, rec)["error"]; got != "internal server error" {
		t.Errorf("error = %q, want a generic message", got)
	}
}
