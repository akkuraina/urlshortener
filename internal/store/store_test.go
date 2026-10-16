package store

import (
	"context"
	"errors"
	"os"
	"testing"

	"urlshortener/internal/shortener"
)

// These are integration tests against a real Postgres. They are skipped
// unless DATABASE_URL points at a database with the migrations applied.
// Test codes are 8 characters, so they can never clash with real 7-character codes.

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL not set; skipping Postgres integration test")
	}
	pool, err := NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return New(pool)
}

// fixedCodes returns a code generator that hands out codes in order.
func fixedCodes(codes ...string) func() (string, error) {
	i := 0
	return func() (string, error) {
		c := codes[i%len(codes)]
		i++
		return c, nil
	}
}

// deleteAfter removes the row for code when the test finishes.
func deleteAfter(t *testing.T, s *Store, code string) {
	t.Helper()
	t.Cleanup(func() {
		// t.Context() is already cancelled during cleanup, so use a fresh one.
		if _, err := s.pool.Exec(context.Background(), "DELETE FROM urls WHERE short_code = $1", code); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
}

func TestCreateAndGet(t *testing.T) {
	s := newTestStore(t)
	s.newCode = fixedCodes("tcreate1")
	deleteAfter(t, s, "tcreate1")

	created, err := s.Create(t.Context(), "https://example.com/long")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 || created.ShortCode != "tcreate1" || created.LongURL != "https://example.com/long" {
		t.Errorf("Create returned %+v", created)
	}
	if created.AccessCount != 0 || created.CreatedAt.IsZero() {
		t.Errorf("Create defaults wrong: %+v", created)
	}

	got, err := s.Get(t.Context(), "tcreate1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != created {
		t.Errorf("Get = %+v, want %+v", got, created)
	}
}

func TestNotFound(t *testing.T) {
	s := newTestStore(t)

	if _, err := s.Get(t.Context(), "tmissing"); !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
	if _, err := s.Visit(t.Context(), "tmissing"); !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Visit error = %v, want ErrNotFound", err)
	}
	if _, err := s.Update(t.Context(), "tmissing", "https://example.com"); !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Update error = %v, want ErrNotFound", err)
	}
	if err := s.Delete(t.Context(), "tmissing"); !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestUpdate(t *testing.T) {
	s := newTestStore(t)
	s.newCode = fixedCodes("tupdate1")
	deleteAfter(t, s, "tupdate1")

	created, err := s.Create(t.Context(), "https://example.com/old")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	updated, err := s.Update(t.Context(), "tupdate1", "https://example.com/new")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.LongURL != "https://example.com/new" {
		t.Errorf("LongURL = %q, want the new URL", updated.LongURL)
	}
	if updated.ID != created.ID || !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("Update changed ID or CreatedAt: before %+v, after %+v", created, updated)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want later than %v", updated.UpdatedAt, created.UpdatedAt)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	s.newCode = fixedCodes("tdelete1")
	deleteAfter(t, s, "tdelete1") // in case the test fails before Delete runs

	if _, err := s.Create(t.Context(), "https://example.com"); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Delete(t.Context(), "tdelete1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := s.Get(t.Context(), "tdelete1"); !errors.Is(err, shortener.ErrNotFound) {
		t.Errorf("Get after Delete error = %v, want ErrNotFound", err)
	}
}

func TestVisitIncrements(t *testing.T) {
	s := newTestStore(t)
	s.newCode = fixedCodes("tvisit01")
	deleteAfter(t, s, "tvisit01")

	created, err := s.Create(t.Context(), "https://example.com")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	var last shortener.URL
	for range 3 {
		if last, err = s.Visit(t.Context(), "tvisit01"); err != nil {
			t.Fatalf("Visit: %v", err)
		}
	}
	if last.AccessCount != 3 {
		t.Errorf("AccessCount = %d, want 3", last.AccessCount)
	}
	if !last.UpdatedAt.Equal(created.UpdatedAt) {
		t.Errorf("Visit changed UpdatedAt from %v to %v", created.UpdatedAt, last.UpdatedAt)
	}
}

func TestCreateRetriesOnCollision(t *testing.T) {
	s := newTestStore(t)
	deleteAfter(t, s, "tcollide")
	deleteAfter(t, s, "tfresh01")

	s.newCode = fixedCodes("tcollide")
	if _, err := s.Create(t.Context(), "https://example.com/first"); err != nil {
		t.Fatalf("first Create: %v", err)
	}

	// The first code is taken, so Create should fall through to the second.
	s.newCode = fixedCodes("tcollide", "tfresh01")
	u, err := s.Create(t.Context(), "https://example.com/second")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if u.ShortCode != "tfresh01" {
		t.Errorf("ShortCode = %q, want %q", u.ShortCode, "tfresh01")
	}
}

func TestCreateGivesUp(t *testing.T) {
	s := newTestStore(t)
	deleteAfter(t, s, "tstuck01")

	s.newCode = fixedCodes("tstuck01")
	if _, err := s.Create(t.Context(), "https://example.com"); err != nil {
		t.Fatalf("first Create: %v", err)
	}
	// Every attempt now collides.
	if _, err := s.Create(t.Context(), "https://example.com"); err == nil {
		t.Fatal("Create succeeded, want error after max attempts")
	}
}
