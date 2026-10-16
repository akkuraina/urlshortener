package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// maxBodyBytes caps request bodies. Valid requests are far smaller.
const maxBodyBytes = 64 << 10 // 64 KiB

// errorResponse is the JSON body for every error: {"error": "message"}.
type errorResponse struct {
	Error string `json:"error"`
}

// readJSON decodes a single JSON object from the request body into dst.
// Its errors have messages that are safe to show to the client.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxErr *json.SyntaxError
		var typeErr *json.UnmarshalTypeError
		var maxErr *http.MaxBytesError

		switch {
		case errors.Is(err, io.EOF):
			return errors.New("request body is empty")
		case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("request body is not valid JSON")
		case errors.As(err, &typeErr):
			return fmt.Errorf("field %q must be a %s", typeErr.Field, typeErr.Type)
		case errors.As(err, &maxErr):
			return fmt.Errorf("request body must be at most %d bytes", maxErr.Limit)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			// encoding/json has no typed error for this, so match the message.
			return errors.New(strings.TrimPrefix(err.Error(), "json: "))
		default:
			return err
		}
	}

	// Reject trailing data such as `{"url":"a"}{"url":"b"}`.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}

// writeJSON sends v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Headers are already sent, so all we can do is log it.
		log.Printf("writing JSON response: %v", err)
	}
}

// writeError sends {"error": msg} with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// serverError logs err and sends a generic 500, so internal details
// (SQL, hostnames, etc.) never leak to the client.
func serverError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s: %v", r.Method, r.URL.Path, err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}
