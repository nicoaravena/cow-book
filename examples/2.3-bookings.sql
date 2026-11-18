-- Step 2.3: the database refuses overlapping bookings of one space.
-- The tenant needs migration 00002. Create a new tenant, or bring an existing
-- one up to date, then pass its schema as the tenant variable:
--
--   make create-tenant SLUG=south-hub NAME="South Hub"
--   make migrate-tenant TENANT=tenant_1
--   docker compose exec -T db psql -U cowbook -d cowbook -v tenant=tenant_1 < examples/2.3-bookings.sql
--
-- Like 2.1 and 2.2, everything is rolled back at the end.

\set ON_ERROR_ROLLBACK on
\set ECHO queries
BEGIN;

SET LOCAL search_path = :"tenant", shared;
-- Show times in Madrid, the location's zone. Stored values don't change.
SET LOCAL timezone = 'Europe/Madrid';

INSERT INTO locations (name, timezone) VALUES ('Centre', 'Europe/Madrid');
INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
SELECT id, 'Room A', 'meeting_room', 6, '2 hours' FROM locations;
INSERT INTO members (name, email) VALUES ('Ana', 'ana@example.com'), ('Bo', 'bo@example.com');

-- Accepted: Ana books Room A, 10:00-11:00 in Madrid.
INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
SELECT s.id, m.id, 2, '2026-12-01 10:00 Europe/Madrid', '2026-12-01 11:00 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'ana@example.com'
RETURNING id, starts_at, ends_at;

-- Refused: Bo wants 10:30-11:30 (bookings_no_overlap, SQLSTATE 23P01).
INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
SELECT s.id, m.id, 1, '2026-12-01 10:30 Europe/Madrid', '2026-12-01 11:30 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'bo@example.com';

-- Accepted: 11:00-12:00 starts when Ana's ends. The range is [start, end).
INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
SELECT s.id, m.id, 1, '2026-12-01 11:00 Europe/Madrid', '2026-12-01 12:00 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'bo@example.com';

-- Accepted: staff member 7 books 15:00-16:00 for Ana. It is pending until
-- she approves, and needs a deadline (bookings_approval_iff_staff).
INSERT INTO bookings (space_id, member_id, booked_by, attendees, starts_at, ends_at, approval_deadline)
SELECT s.id, m.id, 7, 1, '2026-12-01 15:00 Europe/Madrid', '2026-12-01 16:00 Europe/Madrid',
       '2026-12-01 15:00 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'ana@example.com';

-- Refused: the pending booking holds its slot (bookings_no_overlap).
INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
SELECT s.id, m.id, 1, '2026-12-01 15:30 Europe/Madrid', '2026-12-01 16:30 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'bo@example.com';

-- Ana cancels her 10:00 booking. The row stays, as history.
UPDATE bookings SET cancelled_at = now()
WHERE starts_at = '2026-12-01 10:00 Europe/Madrid';

-- Accepted now: the constraint ignores cancelled bookings.
INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
SELECT s.id, m.id, 1, '2026-12-01 10:00 Europe/Madrid', '2026-12-01 10:45 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'bo@example.com';

-- Refused: ends before it starts (bookings_starts_before_ends).
INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
SELECT s.id, m.id, 1, '2026-12-01 13:00 Europe/Madrid', '2026-12-01 12:00 Europe/Madrid'
FROM spaces s, members m WHERE m.email = 'bo@example.com';

-- Refused: a member booking has nothing to approve (bookings_approved_needs_deadline).
UPDATE bookings SET approved_at = now() WHERE booked_by IS NULL;

-- Refused: Ana has bookings, so her row can't go (bookings_member_id_fkey).
-- Members are soft-deleted instead.
DELETE FROM members WHERE email = 'ana@example.com';

SELECT b.id, m.name, b.starts_at, b.ends_at, b.booked_by,
       b.cancelled_at IS NOT NULL AS cancelled
FROM bookings b JOIN members m ON m.id = b.member_id
ORDER BY b.starts_at, b.id;

ROLLBACK;
