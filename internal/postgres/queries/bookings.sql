-- name: CreateBooking :one
-- CreateBooking adds a booking and returns its id. bookings_no_overlap refuses
-- it with 23P01 if it overlaps a booking of the space that isn't cancelled.
-- The timestamps come from the caller, as in CreateSpace.
INSERT INTO bookings (
    space_id, member_id, booked_by, attendees, starts_at, ends_at,
    approval_deadline, approved_at, cancelled_at, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id;

-- name: ExpireLapsedPending :execrows
-- ExpireLapsedPending cancels, at @now, the pending bookings of the space whose
-- approval deadline has passed by @now and that overlap [@starts_at, @ends_at).
-- The constraint's WHERE can't use now(), so this is how a lapsed booking lets
-- go of its slot. cancelled_at IS NULL and the range match the constraint's
-- partial gist index, which finds the rows.
UPDATE bookings
SET cancelled_at = @now, updated_at = @now
WHERE space_id = @space_id
  AND cancelled_at IS NULL
  AND approved_at IS NULL
  AND approval_deadline <= @now
  AND tstzrange(starts_at, ends_at) && tstzrange(@starts_at::timestamptz, @ends_at::timestamptz);

-- name: GetBooking :one
-- GetBooking returns pgx.ErrNoRows if the tenant has no such booking.
SELECT * FROM bookings WHERE id = $1;

-- name: ListBookings :many
-- ListBookings returns every booking, cancelled ones included. A lapsed
-- pending booking stays uncancelled until lazy expiry reaches it, so callers
-- read its state with Booking.State.
SELECT * FROM bookings ORDER BY id;

-- name: ApproveBooking :execrows
-- ApproveBooking records the approval only while the booking is pending: not
-- cancelled, not approved, and with a deadline still ahead of @approved_at.
-- A member booking has a NULL deadline, which the comparison never matches.
-- It changes 0 rows otherwise; the caller reads the row to tell why.
UPDATE bookings
SET approved_at = @approved_at, updated_at = @updated_at
WHERE id = @id
  AND cancelled_at IS NULL
  AND approved_at IS NULL
  AND approval_deadline > @approved_at;

-- name: CancelBooking :execrows
-- CancelBooking records the cancellation only while the booking isn't
-- cancelled yet, so of two racing cancels one changes the row and the other
-- changes 0 rows. It writes only the cancel columns: an approval saved in
-- between keeps its approved_at.
UPDATE bookings
SET cancelled_at = @cancelled_at, updated_at = @updated_at
WHERE id = @id
  AND cancelled_at IS NULL;

-- name: ListMemberBookingsEndingAfter :many
-- ListMemberBookingsEndingAfter returns the member's bookings that aren't
-- cancelled and end after @after, the ones deleting the member cancels.
-- bookings_member_id_idx finds them.
SELECT * FROM bookings
WHERE member_id = @member_id
  AND cancelled_at IS NULL
  AND ends_at > @after
ORDER BY starts_at, id;

-- name: CancelMemberBookingsEndingAfter :execrows
-- CancelMemberBookingsEndingAfter cancels, at @now, every booking of the
-- member that isn't cancelled and hasn't ended by @now: the rule of
-- ListMemberBookingsEndingAfter and of Booking.cancel. Member deletion runs it
-- last, for bookings made after the service listed them.
UPDATE bookings
SET cancelled_at = @now, updated_at = @now
WHERE member_id = @member_id
  AND cancelled_at IS NULL
  AND ends_at > @now;
