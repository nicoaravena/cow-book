-- Tenant migration: bookings. A new file rather than an edit of 00001, which
-- tenants have already applied: goose never runs a version twice.

-- +goose Up
CREATE TABLE bookings (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    -- No cascade on either reference: a booking is history, so a space or
    -- member with bookings can't be deleted. Members are soft-deleted.
    space_id          bigint NOT NULL REFERENCES spaces,
    member_id         bigint NOT NULL REFERENCES members,
    -- The staff member who booked for the member, NULL when the member booked
    -- for themselves. No foreign key until the staff table exists (M4).
    booked_by         bigint,
    -- Capacity is checked by cowbook.NewBooking: a check constraint can't read
    -- the space's row.
    attendees         integer NOT NULL
                      CONSTRAINT bookings_attendees_positive CHECK (attendees >= 1),
    starts_at         timestamptz NOT NULL,
    ends_at           timestamptz NOT NULL, -- not included, as Booking.End
    approval_deadline timestamptz, -- NULL when no approval is needed
    approved_at       timestamptz, -- NULL until the member approves
    cancelled_at      timestamptz, -- NULL until cancelled or declined
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT bookings_starts_before_ends CHECK (starts_at < ends_at),
    -- Every staff booking needs the member's approval, and only staff ones do.
    CONSTRAINT bookings_approval_iff_staff CHECK (
        (booked_by IS NULL) = (approval_deadline IS NULL)
    ),
    CONSTRAINT bookings_approved_needs_deadline CHECK (
        approved_at IS NULL OR approval_deadline IS NOT NULL
    ),
    -- No two bookings for one space overlap, unless one is cancelled. The
    -- range is [starts_at, ends_at), so a booking may start when another ends.
    -- Pending bookings hold their slot, even past their deadline: the WHERE
    -- can't use now(), so lapsed ones are cancelled lazily (2.5). btree_gist,
    -- in shared, gives gist the = on space_id.
    CONSTRAINT bookings_no_overlap EXCLUDE USING gist (
        space_id WITH =,
        tstzrange(starts_at, ends_at) WITH &&
    ) WHERE (cancelled_at IS NULL)
);

-- A member's bookings: "my bookings" (3.6) and member deletion (2.6). Lookups
-- by space use the constraint's gist index.
CREATE INDEX bookings_member_id_idx ON bookings (member_id);

-- +goose Down
DROP TABLE bookings;
