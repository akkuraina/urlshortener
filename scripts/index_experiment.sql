-- Indexing experiment: how much does urls_short_code_key matter?
--
-- Run with:  psql urlshortener -X -f scripts/index_experiment.sql
--
-- Everything runs in one transaction that ends in ROLLBACK, so the table,
-- its rows and its index are left exactly as they were. The ALTER TABLE
-- locks the table until then, so don't run this while the server is busy.

\set ON_ERROR_STOP on
\pset pager off
BEGIN;

\echo
\echo '=== 0. Tiny table: the planner may skip the index entirely ==='
ANALYZE urls;
EXPLAIN SELECT * FROM urls WHERE short_code = 'x0765432';

\echo
\echo '=== 1. Insert 1,000,000 rows WITH the index (note the time) ==='
-- 8-character codes, so they can never match a real 7-character code.
\timing on
INSERT INTO urls (url, short_code)
SELECT 'https://example.com/page/' || g, 'x' || lpad(g::text, 7, '0')
FROM generate_series(1, 1000000) AS g;
\timing off
ANALYZE urls;

\echo
\echo '=== 2. Lookup by short_code WITH the index ==='
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM urls WHERE short_code = 'x0765432';

\echo
\echo '=== 3. What the index costs in disk space ==='
SELECT pg_size_pretty(pg_relation_size('urls'))                AS table_size,
       pg_size_pretty(pg_relation_size('urls_short_code_key')) AS short_code_index,
       pg_size_pretty(pg_relation_size('urls_pkey'))           AS pkey_index;

\echo
\echo '=== 4. Same lookup WITHOUT the index ==='
ALTER TABLE urls DROP CONSTRAINT urls_short_code_key;
EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM urls WHERE short_code = 'x0765432';

\echo
\echo '=== 5. Insert another 1,000,000 rows WITHOUT the index (compare with 1) ==='
\timing on
INSERT INTO urls (url, short_code)
SELECT 'https://example.com/page/' || g, 'y' || lpad(g::text, 7, '0')
FROM generate_series(1, 1000000) AS g;
\timing off

ROLLBACK;
\echo
\echo 'Rolled back: urls and urls_short_code_key are unchanged.'
