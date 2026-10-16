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
	Visit(ctx context.Context, code string) (shortener.URL, error)
}

// API holds the dependencies shared by all handlers.
type API struct {
	store Store
}

// New returns an API backed by store.
func New(store Store) *API {
	return &API{store: store}
}

// Routes returns the HTTP handler with every endpoint registered.
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", handleHealth)
	mux.HandleFunc("POST /shorten", a.handleCreate)
	mux.HandleFunc("GET /shorten/{code}", a.handleGet)
	return mux
}
