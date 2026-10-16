// Package store persists shortener.URL values in PostgreSQL using pgx.
package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"urlshortener/internal/shortener"
)

const (
	// uniqueViolation is the Postgres error code for a UNIQUE constraint failure.
	uniqueViolation = "23505"
	// shortCodeConstraint must match the constraint name in migrations/0001.
	shortCodeConstraint = "urls_short_code_key"
	// maxCodeAttempts bounds the retry loop if generated codes keep colliding.
	maxCodeAttempts = 5
)

// columns is the SELECT/RETURNING list that scanURL expects, in this order.
const columns = "id, url, short_code, access_count, created_at, updated_at"

// NewPool connects to Postgres and verifies the connection with a ping,
// so a bad DATABASE_URL fails at startup instead of on the first request.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("creating pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connecting to database: %w", err)
	}
	return pool, nil
}

// Store reads and writes URLs. Create one with New.
type Store struct {
	pool *pgxpool.Pool
	// newCode generates short codes. Tests replace it to force collisions.
	newCode func() (string, error)
}

// New returns a Store backed by pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, newCode: shortener.NewCode}
}

// Create stores longURL under a fresh random short code. If the code is
// already taken, it retries with a new one, up to maxCodeAttempts times.
func (s *Store) Create(ctx context.Context, longURL string) (shortener.URL, error) {
	const q = `INSERT INTO urls (url, short_code) VALUES ($1, $2) RETURNING ` + columns

	for range maxCodeAttempts {
		code, err := s.newCode()
		if err != nil {
			return shortener.URL{}, err
		}

		u, err := scanURL(s.pool.QueryRow(ctx, q, longURL, code))
		if isUniqueViolation(err, shortCodeConstraint) {
			continue // code collision: try another code
		}
		if err != nil {
			return shortener.URL{}, fmt.Errorf("inserting url: %w", err)
		}
		return u, nil
	}
	return shortener.URL{}, fmt.Errorf("no free short code after %d attempts", maxCodeAttempts)
}

// Get returns the URL for code without changing it.
// It returns shortener.ErrNotFound if no such code exists.
func (s *Store) Get(ctx context.Context, code string) (shortener.URL, error) {
	const q = `SELECT ` + columns + ` FROM urls WHERE short_code = $1`

	u, err := scanURL(s.pool.QueryRow(ctx, q, code))
	if err != nil {
		return shortener.URL{}, notFoundOr(err, "getting url")
	}
	return u, nil
}

// Visit increments the access count for code and returns the updated URL.
// It returns shortener.ErrNotFound if no such code exists.
func (s *Store) Visit(ctx context.Context, code string) (shortener.URL, error) {
	// A single atomic UPDATE: concurrent visits can't lose increments, the way
	// a separate SELECT followed by an UPDATE could. updated_at is left alone
	// because it tracks edits to the URL, not visits.
	const q = `UPDATE urls SET access_count = access_count + 1
		WHERE short_code = $1 RETURNING ` + columns

	u, err := scanURL(s.pool.QueryRow(ctx, q, code))
	if err != nil {
		return shortener.URL{}, notFoundOr(err, "visiting url")
	}
	return u, nil
}

// Update points code at a new longURL and bumps updated_at.
// It returns shortener.ErrNotFound if no such code exists.
func (s *Store) Update(ctx context.Context, code, longURL string) (shortener.URL, error) {
	const q = `UPDATE urls SET url = $2, updated_at = now()
		WHERE short_code = $1 RETURNING ` + columns

	u, err := scanURL(s.pool.QueryRow(ctx, q, code, longURL))
	if err != nil {
		return shortener.URL{}, notFoundOr(err, "updating url")
	}
	return u, nil
}

// Delete removes code. It returns shortener.ErrNotFound if no such code exists.
func (s *Store) Delete(ctx context.Context, code string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM urls WHERE short_code = $1`, code)
	if err != nil {
		return fmt.Errorf("deleting url: %w", err)
	}
	// Exec returns no rows, so "not found" shows up as zero rows affected.
	if tag.RowsAffected() == 0 {
		return shortener.ErrNotFound
	}
	return nil
}

// scanURL reads one row in the order given by columns.
func scanURL(row pgx.Row) (shortener.URL, error) {
	var u shortener.URL
	err := row.Scan(&u.ID, &u.LongURL, &u.ShortCode, &u.AccessCount, &u.CreatedAt, &u.UpdatedAt)
	return u, err
}

// notFoundOr turns pgx.ErrNoRows into shortener.ErrNotFound and wraps any other error.
func notFoundOr(err error, action string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return shortener.ErrNotFound
	}
	return fmt.Errorf("%s: %w", action, err)
}

// isUniqueViolation reports whether err is a Postgres unique violation on constraint.
func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == uniqueViolation &&
		pgErr.ConstraintName == constraint
}
