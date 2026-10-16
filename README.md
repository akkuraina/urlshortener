# URL Shortener

A small URL shortening service written in Go: a REST API, a PostgreSQL store,
and a minimal web page, all built into a single binary.

It is built with the Go standard library only (no web framework), using the
method-and-path routing added in Go 1.22, plus [pgx](https://github.com/jackc/pgx)
for PostgreSQL. It implements the
[roadmap.sh URL Shortening Service](https://roadmap.sh/projects/url-shortening-service)
spec.

## Features

- Create, read, update and delete short URLs through a JSON REST API
- Redirect endpoint (`GET /{code}`) that sends browsers to the original URL
- Access counting and a statistics endpoint
- URL validation: only absolute `http`/`https` URLs, so `javascript:` links are rejected
- Collision-safe random 7-character base62 codes, enforced by a unique index
- Consistent JSON errors (`{"error": "..."}`), including for unknown routes and wrong methods
- Request logging, server timeouts and graceful shutdown
- A plain HTML/JS frontend embedded in the binary with `go:embed`

## Tech stack

| Layer    | Choice                                                     |
|----------|------------------------------------------------------------|
| Language | Go 1.22+ (developed on 1.25)                               |
| HTTP     | `net/http` with `http.ServeMux` patterns                   |
| Database | PostgreSQL (tested on 16) via `github.com/jackc/pgx/v5`    |
| Frontend | One HTML page and vanilla JavaScript, no build step        |

`pgx` is the only third-party dependency.

## Project structure

```
cmd/server/          Entry point: loads config, connects to the DB, starts the server
internal/config/     Reads settings from environment variables
internal/shortener/  Domain: URL type, validation, short-code generator, errors
internal/store/      PostgreSQL queries (pgx)
internal/httpapi/    Routes, handlers, JSON helpers, middleware
migrations/          SQL schema
scripts/             Indexing experiment (EXPLAIN ANALYZE)
web/                 Frontend files, embedded into the binary
```

Dependencies point one way: `httpapi` and `store` both depend on `shortener`,
but not on each other. `httpapi` declares the small `Store` interface it needs,
which lets the handlers be tested against an in-memory fake.

## Getting started

### Prerequisites

- Go 1.22 or newer
- PostgreSQL

On Ubuntu or WSL:

```bash
sudo apt install postgresql
sudo service postgresql start
sudo -u postgres createuser --superuser $USER
```

### Set up the database

```bash
createdb urlshortener
psql urlshortener -f migrations/0001_create_urls.up.sql
```

To undo the schema, run `migrations/0001_create_urls.down.sql`.

### Run the server

```bash
export DATABASE_URL=postgres:///urlshortener
go run ./cmd/server
```

Then open **http://localhost:8080** to use the frontend.

To build a standalone binary (the frontend is included):

```bash
go build -o urlshortener ./cmd/server
./urlshortener
```

### Configuration

| Variable       | Required | Default | Description                                                        |
|----------------|----------|---------|--------------------------------------------------------------------|
| `DATABASE_URL` | yes      |         | PostgreSQL connection string, e.g. `postgres://user:pass@host:5432/db` |
| `PORT`         | no       | `8080`  | Port to listen on                                                  |

`postgres:///urlshortener` (three slashes, no host) connects through the local
Unix socket as your OS user, with no password.

## API

All request and response bodies are JSON. Errors always have the form
`{"error": "message"}`.

| Method   | Path                    | Description                         | Success | Errors   |
|----------|-------------------------|-------------------------------------|---------|----------|
| `POST`   | `/shorten`              | Create a short URL                  | 201     | 400      |
| `GET`    | `/shorten/{code}`       | Get the original URL (counts as an access) | 200 | 404 |
| `PUT`    | `/shorten/{code}`       | Change the original URL             | 200     | 400, 404 |
| `DELETE` | `/shorten/{code}`       | Delete a short URL                  | 204     | 404      |
| `GET`    | `/shorten/{code}/stats` | Get statistics (not counted)        | 200     | 404      |
| `GET`    | `/{code}`               | 302 redirect to the original URL (counted) | 302 | 404 |
| `GET`    | `/healthz`              | Health check                        | 200     |          |

### Create a short URL

```bash
curl -i -X POST localhost:8080/shorten \
  -d '{"url": "https://www.example.com/some/long/url"}'
```

```http
HTTP/1.1 201 Created
Location: /shorten/abc1234
Content-Type: application/json

{
  "id": "1",
  "url": "https://www.example.com/some/long/url",
  "shortCode": "abc1234",
  "createdAt": "2021-09-01T12:00:00Z",
  "updatedAt": "2021-09-01T12:00:00Z"
}
```

### Retrieve the original URL

```bash
curl localhost:8080/shorten/abc1234
```

The response is the same shape as above. Each call increases the access count.

### Update a short URL

```bash
curl -X PUT localhost:8080/shorten/abc1234 \
  -d '{"url": "https://www.example.com/some/updated/url"}'
```

The response is the updated object, with a new `updatedAt`.

### Delete a short URL

```bash
curl -i -X DELETE localhost:8080/shorten/abc1234   # 204 No Content
```

### Get statistics

```bash
curl localhost:8080/shorten/abc1234/stats
```

```json
{
  "id": "1",
  "url": "https://www.example.com/some/long/url",
  "shortCode": "abc1234",
  "createdAt": "2021-09-01T12:00:00Z",
  "updatedAt": "2021-09-01T12:00:00Z",
  "accessCount": 10
}
```

### Follow a short link

```bash
curl -i localhost:8080/abc1234
# HTTP/1.1 302 Found
# Location: https://www.example.com/some/long/url
```

The redirect uses 302 rather than 301, so browsers don't cache it. That way
every visit is counted, and an update takes effect immediately.

### Validation errors

```bash
curl -X POST localhost:8080/shorten -d '{"url": "example.com"}'
# 400 {"error":"invalid url: must start with http:// or https://"}
```

Other 400 responses cover an empty body, malformed JSON, unknown fields,
wrong field types, URLs longer than 2048 characters, and bodies over 64 KiB.

## Database

```sql
CREATE TABLE urls (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url          TEXT        NOT NULL,
    short_code   VARCHAR(16) NOT NULL,
    access_count BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT urls_short_code_key UNIQUE (short_code),
    CONSTRAINT urls_access_count_nonnegative CHECK (access_count >= 0)
);
```

- The `UNIQUE` constraint creates a B-tree index on `short_code`. Every lookup,
  update, delete and redirect uses it. It also guarantees codes are unique: on a
  collision, the store sees the unique-violation error and retries with a new code.
- Visits are counted with a single atomic
  `UPDATE ... SET access_count = access_count + 1`, so concurrent requests
  never lose counts.
- `url` and `created_at` are deliberately not indexed, because no query filters
  or sorts on them. Each index slows down every write.

To inspect the data:

```bash
psql urlshortener -c '\d urls'
psql urlshortener -c 'SELECT short_code, url, access_count FROM urls ORDER BY id'
```

### Indexing experiment

`scripts/index_experiment.sql` inserts 1,000,000 rows and compares a lookup
with and without the `short_code` index, using `EXPLAIN (ANALYZE, BUFFERS)`.
It runs inside a transaction that is rolled back, so your data is left unchanged.

```bash
psql urlshortener -X -f scripts/index_experiment.sql
```

Example results on a laptop:

|                         | With index                  | Without index                 |
|-------------------------|-----------------------------|-------------------------------|
| Query plan              | Index Scan                  | Parallel Seq Scan (3 processes) |
| Lookup time             | 0.02 ms                     | 37 ms                         |
| 8 KB pages read         | 4                           | 13,235                        |
| Insert of 1M rows       | 3.5 s                       | 2.2 s                         |
| Extra disk space        | 30 MB                       | 0                             |

The index makes reads roughly 1,800 times faster in exchange for slower writes
and some disk space. That's a good trade for a service that is read far more
often than it is written.

## Testing

```bash
go test ./...
```

- `internal/shortener`: unit tests for URL validation and code generation.
- `internal/httpapi`: handler tests that run every endpoint through the real
  router and middleware, using `httptest` and an in-memory fake store. These
  tests need no database.
- `internal/store`: integration tests against a real PostgreSQL. They're
  skipped unless `DATABASE_URL` is set, and they clean up the rows they create.

```bash
DATABASE_URL=postgres:///urlshortener go test -v ./internal/store/
go test -cover ./...
```

## Possible improvements

- A `url_visits` table for per-day statistics
- Structured logging with `log/slog`
- A migration tool and a Dockerfile
- Rate limiting on `POST /shorten`
