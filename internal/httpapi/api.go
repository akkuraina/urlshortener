// Package httpapi exposes the URL shortener over HTTP: routing, request
// decoding, response encoding, and mapping domain errors to status codes.
package httpapi

import (
	"context"
	"net/http"

	"urlshortener/internal/shortener"
)

// Store is the storage this package needs. It is defined here, where it is
// used, so handlers can be tested with a fake. *store.Store satisfies it.
type Store interface {
	Create(ctx context.Context, longURL string) (shortener.URL, error)
	Get(ctx context.Context, code string) (shortener.URL, error)
	Visit(ctx context.Context, code string) (shortener.URL, error)
	Update(ctx context.Context, code, longURL string) (shortener.URL, error)
	Delete(ctx context.Context, code string) error
}

// API holds the dependencies shared by all handlers.
type API struct {
	store Store
}

// New returns an API backed by store.
func New(store Store) *API {
	return &API{store: store}
}

// Routes returns the HTTP handler with every endpoint registered,
// wrapped in middleware.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /shorten", a.handleCreate)
	mux.HandleFunc("GET /shorten/{code}", a.handleGet)
	mux.HandleFunc("PUT /shorten/{code}", a.handleUpdate)
	mux.HandleFunc("DELETE /shorten/{code}", a.handleDelete)
	mux.HandleFunc("GET /shorten/{code}/stats", a.handleStats)

	// Matches any single-segment path. More specific patterns such as
	// "GET /healthz" win, regardless of registration order.
	mux.HandleFunc("GET /{code}", a.handleRedirect)

	// Middleware runs outside-in: logRequests sees the final status.
	return logRequests(jsonFallback(mux))
}
