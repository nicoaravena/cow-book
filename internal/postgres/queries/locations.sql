-- name: CreateLocation :one
-- CreateLocation adds a location and returns its id. Like every query here it
-- runs inside WithTenant, so the table name stays unqualified and the
-- transaction's search_path picks the tenant's schema.
INSERT INTO locations (name, timezone) VALUES ($1, $2) RETURNING id;

-- name: GetLocation :one
-- GetLocation returns pgx.ErrNoRows if the tenant has no such location.
SELECT id, name, timezone FROM locations WHERE id = $1;

-- name: GetLocationByTimezone :one
-- GetLocationByTimezone returns the first location in the timezone, or
-- pgx.ErrNoRows if there is none. Nothing stops two locations sharing a zone,
-- so the lowest id wins.
SELECT id FROM locations WHERE timezone = $1 ORDER BY id LIMIT 1;
