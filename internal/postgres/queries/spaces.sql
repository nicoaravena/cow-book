-- name: CreateSpace :one
-- CreateSpace adds a space and returns its id. The timestamps come from the
-- caller, so the domain's clock decides them rather than now().
INSERT INTO spaces (
    location_id, name, kind, capacity, max_booking_length, available, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id;

-- name: CreateOpeningWindow :exec
INSERT INTO opening_windows (space_id, weekday, opens_min, closes_min) VALUES ($1, $2, $3, $4);

-- name: GetSpace :one
-- GetSpace returns pgx.ErrNoRows if the tenant has no such space.
-- sqlc.embed puts the space's columns in one tenantdb.Space field of the row.
SELECT sqlc.embed(spaces), locations.timezone
FROM spaces
JOIN locations ON locations.id = spaces.location_id
WHERE spaces.id = $1;

-- name: ListSpaces :many
SELECT * FROM spaces ORDER BY id;

-- name: ListOpeningWindows :many
SELECT * FROM opening_windows WHERE space_id = $1 ORDER BY weekday, opens_min;

-- name: ListAllOpeningWindows :many
-- ListAllOpeningWindows returns every space's windows, so a list of spaces
-- needs two queries rather than one per space.
SELECT * FROM opening_windows ORDER BY space_id, weekday, opens_min;
