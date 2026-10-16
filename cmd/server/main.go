// Command server runs the URL shortener HTTP API.
package main

import (
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()

	// Go 1.22+ patterns: "METHOD /path". Other methods on this path get 405 automatically.
	mux.HandleFunc("GET /healthz", handleHealth)

	addr := ":8080"
	log.Printf("listening on %s", addr)

	// ListenAndServe blocks until the server fails, and it always returns a non-nil error.
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
