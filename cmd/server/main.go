// Command server runs the URL shortener HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"urlshortener/internal/config"
)

// shutdownTimeout is how long in-flight requests get to finish after Ctrl+C.
const shutdownTimeout = 10 * time.Second

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

// run holds the real startup logic. Returning an error (instead of calling
// log.Fatal everywhere) lets deferred cleanup run and keeps main trivial.
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// ctx is cancelled when the process receives Ctrl+C (SIGINT) or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mux := http.NewServeMux()
	// Go 1.22+ patterns: "METHOD /path". Other methods on this path get 405 automatically.
	mux.HandleFunc("GET /healthz", handleHealth)

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// ListenAndServe blocks, so run it in a goroutine and report its result on a channel.
	// The buffer of 1 lets the goroutine send and exit even if nobody is receiving.
	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	// Wait for whichever happens first: the server fails, or we get a signal.
	select {
	case err := <-errCh:
		return fmt.Errorf("server: %w", err)
	case <-ctx.Done():
	}

	// Restore default signal handling so a second Ctrl+C force-quits.
	stop()
	log.Println("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	// Shutdown stops accepting new connections and waits for active requests to finish.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	// After Shutdown, ListenAndServe returns http.ErrServerClosed, which is expected.
	if err := <-errCh; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("server: %w", err)
	}

	log.Println("server stopped")
	return nil
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}
