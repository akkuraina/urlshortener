package httpapi

import (
	"errors"
	"net/http"
	"time"

	"urlshortener/internal/shortener"
)

// urlRequest is the body of POST /shorten (and later PUT /shorten/{code}).
type urlRequest struct {
	URL string `json:"url"`
}

// urlResponse is the JSON shape of a short URL, as given in the spec.
type urlResponse struct {
	ID        int64     `json:"id,string"` // the spec sends the id as a string: "1"
	URL       string    `json:"url"`
	ShortCode string    `json:"shortCode"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func toURLResponse(u shortener.URL) urlResponse {
	return urlResponse{
		ID:        u.ID,
		URL:       u.LongURL,
		ShortCode: u.ShortCode,
		CreatedAt: u.CreatedAt.UTC(),
		UpdatedAt: u.UpdatedAt.UTC(),
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleCreate serves POST /shorten.
func (a *API) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req urlRequest
	if err := readJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	longURL, err := shortener.ValidateURL(req.URL)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	u, err := a.store.Create(r.Context(), longURL)
	if err != nil {
		serverError(w, r, err)
		return
	}

	w.Header().Set("Location", "/shorten/"+u.ShortCode)
	writeJSON(w, http.StatusCreated, toURLResponse(u))
}

// handleGet serves GET /shorten/{code}. Each call counts as an access.
func (a *API) handleGet(w http.ResponseWriter, r *http.Request) {
	u, err := a.store.Visit(r.Context(), r.PathValue("code"))
	if errors.Is(err, shortener.ErrNotFound) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if err != nil {
		serverError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, toURLResponse(u))
}
