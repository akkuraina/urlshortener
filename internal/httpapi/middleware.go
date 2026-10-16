package httpapi

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// logRequests logs one line per request: method, path, status, duration.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		log.Printf("%s %s %d %s", r.Method, r.URL.Path, rec.status, time.Since(start).Round(time.Microsecond))
	})
}

// statusRecorder wraps a ResponseWriter to remember the status code.
// Embedding the interface forwards every other method (Header, Write) unchanged.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (rec *statusRecorder) Unwrap() http.ResponseWriter {
	return rec.ResponseWriter
}

// jsonFallback makes the mux's own 404 and 405 replies (for requests that
// match no route) use our JSON error shape instead of plain text.
func jsonFallback(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// An empty pattern means no route matched, so the mux will reply
		// with its built-in 404 or 405 handler.
		if _, pattern := mux.Handler(r); pattern == "" {
			w = &jsonErrorWriter{ResponseWriter: w}
		}
		mux.ServeHTTP(w, r)
	})
}

// jsonErrorWriter replaces a plain-text error body with {"error": "..."}.
// Headers the mux already set, such as Allow on a 405, are kept.
type jsonErrorWriter struct {
	http.ResponseWriter
	wroteJSON bool
}

func (w *jsonErrorWriter) WriteHeader(status int) {
	w.Header().Set("Content-Type", "application/json")
	w.ResponseWriter.WriteHeader(status)
	json.NewEncoder(w.ResponseWriter).Encode(errorResponse{Error: http.StatusText(status)})
	w.wroteJSON = true
}

// Write discards the mux's plain-text body once the JSON body is written.
func (w *jsonErrorWriter) Write(b []byte) (int, error) {
	if w.wroteJSON {
		return len(b), nil
	}
	return w.ResponseWriter.Write(b)
}
