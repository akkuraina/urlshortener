-- 0001: the urls table, one row per short link.

CREATE TABLE urls (
    -- Surrogate key. Identity columns are the modern, SQL-standard replacement for SERIAL.
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url          TEXT        NOT NULL,
    short_code   VARCHAR(16) NOT NULL,
    access_count BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- No trigger: the UPDATE query sets this explicitly (updated_at = now()).
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Serves: every lookup by code (GET/PUT/DELETE /shorten/{code}, stats, redirect).
    -- Also guarantees no two rows share a code. The Go code checks for this
    -- constraint name to detect a code collision and retry with a new code.
    CONSTRAINT urls_short_code_key UNIQUE (short_code),

    CONSTRAINT urls_access_count_nonnegative CHECK (access_count >= 0)
);

-- Deliberately NOT indexed:
--   url        - no query filters by it. Add an index only if we dedupe long URLs.
--   created_at - no listing/sorting endpoint yet.
-- Each index speeds up reads on its column but slows every INSERT/UPDATE and uses disk.
