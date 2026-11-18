-- Step 2.2: a tenant schema's tables accept good rows and refuse bad ones.
-- Create a tenant first, then pass its schema as the tenant variable:
--
--   make create-tenant SLUG=north-hub NAME="North Hub"
--   docker compose exec -T db psql -U cowbook -d cowbook -v tenant=tenant_1 < examples/2.2-tenant.sql
--
-- Like 2.1, everything is rolled back at the end.

\set ON_ERROR_ROLLBACK on
\set ECHO queries
BEGIN;

-- Unqualified names now mean the tenant's tables, as they will for the app.
-- :"tenant" quotes the variable as an identifier.
SET LOCAL search_path = :"tenant", shared;

INSERT INTO locations (name, timezone) VALUES ('Centre', 'Europe/Madrid') RETURNING id;

INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
SELECT id, 'Room A', 'meeting_room', 6, '2 hours' FROM locations
RETURNING id, name, kind, available;

-- Monday 09:00-13:00 and 14:00-18:00. Weekdays follow Go: Sunday is 0.
INSERT INTO opening_windows (space_id, weekday, opens_min, closes_min)
SELECT id, 1, 540, 780 FROM spaces UNION ALL
SELECT id, 1, 840, 1080 FROM spaces;

INSERT INTO members (name, email) VALUES ('Ana', 'ana@example.com') RETURNING id, email;

-- Refused: not a space kind (spaces_kind_valid).
INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
SELECT id, 'Sauna', 'sauna', 4, '1 hour' FROM locations;

-- Refused: nobody fits (spaces_capacity_positive).
INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
SELECT id, 'Cupboard', 'phone_booth', 0, '1 hour' FROM locations;

-- Refused: longer than a day (spaces_max_booking_length_valid).
INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
SELECT id, 'Desk 1', 'hot_desk', 1, '25 hours' FROM locations;

-- Refused: no such location (spaces_location_id_fkey).
INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
VALUES (999, 'Desk 1', 'hot_desk', 1, '8 hours');

-- Refused: there is no weekday 7 (opening_windows_weekday_valid).
INSERT INTO opening_windows SELECT id, 7, 540, 1080 FROM spaces;

-- Refused: opens after it closes (opening_windows_opens_before_closes).
INSERT INTO opening_windows SELECT id, 2, 1080, 540 FROM spaces;

-- Refused: emails are stored lowercased (members_email_normalized).
INSERT INTO members (name, email) VALUES ('Bo', 'Bo@example.com');

-- Refused: a member who isn't deleted has this email (members_email_key).
INSERT INTO members (name, email) VALUES ('Ana Again', 'ana@example.com');

-- Accepted: once Ana is deleted, the email can register again.
UPDATE members SET deleted_at = now() WHERE email = 'ana@example.com';
INSERT INTO members (name, email) VALUES ('Ana Again', 'ana@example.com');

SELECT id, name, email, deleted_at IS NOT NULL AS deleted FROM members ORDER BY id;

\dt
ROLLBACK;
