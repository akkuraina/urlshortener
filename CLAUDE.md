# URL Shortener — Go learning project

A URL shortener REST API built in Go to practice idiomatic Go. The goals are
**idiomatic project structure**, **clean API endpoints**, and **database indexing**.

## How to work with me (the user)

- Work in **small steps, one at a time**.
- After each step, **stop and explain** what was written and why. Call out
  Go-specific idioms in particular: error handling, interfaces, contexts,
  struct tags, zero values, package layout, and so on.
- **Do not start the next step until I say so.**
- I'm learning Go, so pick clarity over cleverness and explain any non-obvious choice.
- **Never run git commands that change history or state** (commit, branch, push, etc.). I handle git myself.

## Tech constraints

- Go 1.22+ standard library `net/http`, using method + path-pattern routing
  (`mux.HandleFunc("GET /shorten/{code}", ...)`, `r.PathValue("code")`).
  **No web frameworks** (no gin, echo, chi, etc.).
- PostgreSQL through `github.com/jackc/pgx/v5` (`pgxpool`). No ORM. Write SQL by hand.
- Frontend: one minimal HTML page with vanilla JS. No build step and no JS frameworks.
- Keep dependencies to a minimum. Reach for the standard library first.

## Project layout

```
cmd/server/main.go        entry point: config, DB pool, routes, graceful shutdown
internal/config/          reads settings from environment variables
internal/shortener/       domain: URL type, sentinel errors, short-code generation
internal/store/           Postgres implementation (pgx) of the storage interface
internal/httpapi/         handlers, routing, JSON helpers, middleware
migrations/               plain SQL migration files (NNNN_name.up.sql / .down.sql)
web/                      index.html + app.js, served by the Go binary via embed
```

## Conventions

- **Errors:** return them, don't panic. Wrap with `fmt.Errorf("doing x: %w", err)`.
  Use sentinel errors (`shortener.ErrNotFound`) and check them with `errors.Is` / `errors.As`.
  Only handlers turn errors into HTTP status codes.
- **Interfaces:** define them where they are consumed (in `httpapi`), keep them
  small, and return concrete types from constructors.
- **Context:** every DB call takes `ctx context.Context` as its first parameter,
  passed down from `r.Context()`.
- **JSON:** request and response fields are camelCase (`shortCode`, `createdAt`,
  `accessCount`). Timestamps are RFC 3339 UTC. Errors use the shape `{"error": "message"}`.
- **DB:** tables and columns are snake_case. Use `TIMESTAMPTZ` for times. Every
  index gets a comment in its migration saying which query it serves.
- **Formatting:** run `gofmt` and `go vet` before calling a step done.

## API (from the roadmap.sh URL-shortener spec)

| Method | Path                    | Success | Errors   |
|--------|-------------------------|---------|----------|
| POST   | `/shorten`              | 201     | 400      |
| GET    | `/shorten/{code}`       | 200     | 404      |
| PUT    | `/shorten/{code}`       | 200     | 400, 404 |
| DELETE | `/shorten/{code}`       | 204     | 404      |
| GET    | `/shorten/{code}/stats` | 200     | 404      |
| GET    | `/{code}`               | 302 redirect (extra endpoint, used by the frontend) | 404 |

`accessCount` goes up on each `GET /shorten/{code}` and on each redirect.

## Commands

```bash
export DATABASE_URL=postgres:///urlshortener          # required by the server and store tests
psql urlshortener -f migrations/0001_create_urls.up.sql   # one-time schema setup
go run ./cmd/server                                   # http://localhost:8080
go test ./...                                         # store tests are skipped if DATABASE_URL is unset
psql urlshortener -X -f scripts/index_experiment.sql  # EXPLAIN demo, fully rolled back
```
