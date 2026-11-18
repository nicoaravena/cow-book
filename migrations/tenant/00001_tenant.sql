-- Tenant migration: run in every tenant schema, tenant_<id>. Table names are
-- unqualified; goose runs with search_path set to the tenant's schema, then
-- shared.

-- +goose Up
CREATE TABLE locations (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL
               CONSTRAINT locations_name_not_blank CHECK (btrim(name) <> ''),
    -- An IANA name such as Europe/Madrid. Go checks it is a real zone: a
    -- check constraint can't look it up.
    timezone   text NOT NULL
               CONSTRAINT locations_timezone_not_blank CHECK (btrim(timezone) <> ''),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE spaces (
    id                 bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    location_id        bigint NOT NULL REFERENCES locations,
    name               text NOT NULL
                       CONSTRAINT spaces_name_not_blank CHECK (btrim(name) <> ''),
    kind               text NOT NULL
                       CONSTRAINT spaces_kind_valid CHECK (
                           kind IN ('hot_desk', 'private_office', 'meeting_room', 'phone_booth')
                       ),
    capacity           integer NOT NULL
                       CONSTRAINT spaces_capacity_positive CHECK (capacity >= 1),
    max_booking_length interval NOT NULL
                       CONSTRAINT spaces_max_booking_length_valid CHECK (
                           max_booking_length > interval '0'
                           AND max_booking_length <= interval '24 hours'
                       ),
    available          boolean NOT NULL DEFAULT true,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now()
);

-- One open period on one weekday. weekday follows Go's time.Weekday (Sunday
-- is 0) and the minutes follow cowbook.ClockTime (1440 is the end of the day).
-- That windows on one day don't overlap or touch is checked by
-- cowbook.NewWeeklyHours.
CREATE TABLE opening_windows (
    space_id   bigint NOT NULL REFERENCES spaces ON DELETE CASCADE,
    weekday    smallint NOT NULL
               CONSTRAINT opening_windows_weekday_valid CHECK (weekday BETWEEN 0 AND 6),
    opens_min  smallint NOT NULL
               CONSTRAINT opening_windows_opens_valid CHECK (opens_min BETWEEN 0 AND 1440),
    closes_min smallint NOT NULL
               CONSTRAINT opening_windows_closes_valid CHECK (closes_min BETWEEN 0 AND 1440),
    CONSTRAINT opening_windows_opens_before_closes CHECK (opens_min < closes_min),
    PRIMARY KEY (space_id, weekday, opens_min)
);

CREATE TABLE members (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name       text NOT NULL
               CONSTRAINT members_name_not_blank CHECK (btrim(name) <> ''),
    -- Stored lowercased and trimmed, as cowbook.NewMember does, so the unique
    -- index below compares like with like.
    email      text NOT NULL
               CONSTRAINT members_email_normalized CHECK (
                   email <> '' AND email = lower(btrim(email))
               ),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz -- NULL until the member is deleted
);

-- Unique among members who aren't deleted, so someone who left can register
-- again with the same email. The old row keeps its history.
CREATE UNIQUE INDEX members_email_key ON members (email) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE members;
DROP TABLE opening_windows;
DROP TABLE spaces;
DROP TABLE locations;
