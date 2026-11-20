# Database

Postgres 18, one database for every operator, one schema per operator. So far only
`cowbook create-tenant` and the integration tests connect to it. Tenant work goes through
`WithTenant` (2.4); `postgres.SpaceStore` (2.4b) and `postgres.BookingStore` (2.5) use it.

## Running it locally

`compose.yaml` starts Postgres as the `db` service. Settings come from `.env`, which is gitignored;
`.env.example` lists the names.

| Variable | Used by | Means |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | compose | Password of the `cowbook` user. Required |
| `POSTGRES_PORT` | compose | Host port, default `5432` |
| `DATABASE_URL` | goose, `cowbook create-tenant`, integration tests | Connection string with the same password |

| Make target | Does |
| --- | --- |
| `make db-up` | Start Postgres and wait until it is healthy |
| `make db-down` | Stop it; data stays in the `pgdata` volume |
| `make db-psql` | Open `psql` inside the container |
| `make migrate-platform` | Apply `migrations/platform` and print the status |
| `make migrate-check` | Up, reset, up for `migrations/platform`, in a throwaway `cowbook_migrate_check` database: proves every Down migration works. Tenant migrations get the same check in `TestTenantMigrationsDownAndUp` |
| `make test-integration` | Tests tagged `integration`, against `DATABASE_URL` |
| `make generate` | Regenerates `internal/postgres/tenantdb` with sqlc. `make lint` runs `sqlc diff` and fails while it is stale |
| `make create-tenant SLUG=... NAME=...` | Runs `cowbook create-tenant` with `.env` loaded |
| `make migrate-tenant TENANT=tenant_<id>` | Applies new `migrations/tenant` versions to one existing tenant; exit 2 on a bad name |

## Schemas

| Schema | Holds |
| --- | --- |
| `platform` | Data about operators |
| `shared` | The `btree_gist` extension, for exclusion constraints |
| `tenant_<id>` | One operator's data, and its own `goose_db_version` |
| `public` | goose's `goose_db_version` table |

## Platform migrations

`migrations/platform/`, run once per database.

| Version | Creates |
| --- | --- |
| `00001_platform.sql` | Schemas `platform` and `shared`, extension `btree_gist`, table `platform.tenants` |

## `platform.tenants`

One row per operator.

| Column | Type | Rule |
| --- | --- | --- |
| `id` | `bigint` | Identity, `GENERATED ALWAYS`: never set by the caller. Names the tenant's schema |
| `slug` | `text` | Unique (`tenants_slug_key`). Lowercase letters and digits in groups joined by single hyphens, 3-40 characters (`tenants_slug_format`) |
| `name` | `text` | Not blank (`tenants_name_not_blank`) |
| `status` | `text` | `provisioning` (default), `active` or `suspended` (`tenants_status_valid`) |
| `created_at` | `timestamptz` | Default `now()` |

## Tenants

`cowbook create-tenant <slug> <name>` calls `postgres.Provisioner.CreateTenant`, which:

1. inserts the `platform.tenants` row, status `provisioning`;
2. creates the schema `tenant_<id>`, named by `postgres.SchemaName` from the id only;
3. runs `migrations/tenant` in it, embedded in the binary as `migrations.Tenant`, with
   `search_path` set to `tenant_<id>, shared`;
4. sets the status to `active`.

If step 2, 3 or 4 fails, it drops the schema and deletes the row.

| Exit code | Means |
| --- | --- |
| `0` | Tenant created; prints its id and schema |
| `1` | `DATABASE_URL` unset, or Postgres refused (slug taken or malformed, migration failed) |
| `2` | Wrong number of arguments |

## Tenant transactions

In `internal/postgres/db.go`.

| Function | Does |
| --- | --- |
| `NewPool(ctx, url)` | Opens a `pgxpool.Pool` with query mode `describe_exec`: types are asked for on every query, never cached, so tenants on different migration versions can share a connection. `cache_statement`, pgx's default, fails there with `cached plan must not change result type` (`0A000`) |
| `WithTenant(ctx, pool, tenantID, fn)` | Begins a transaction, sets `search_path` to `tenant_<id>, shared` with `set_config(..., true)`, calls `fn` with the generated queries bound to it, and commits if `fn` returns nil. An error from `fn` comes back wrapped (`errors.Is` finds it); an error or a panic rolls back |

The `search_path` is transaction-local, so the connection goes back to the pool without it.
Outside `WithTenant` tenant tables aren't visible (`42P01`). A tenant without a schema sees no
tables either. `TestWithTenantSearchPathDoesNotCarryOver` proves it on one reused connection.

Queries live in `internal/postgres/queries/*.sql`, run with unqualified table names. sqlc
(`sqlc.yaml`) checks them against `migrations/tenant` and generates package `tenantdb`:

| Query | Returns |
| --- | --- |
| `CreateLocation(ctx, {Name, Timezone})` | The new location's id |
| `GetLocation(ctx, id)` | `id`, `name`, `timezone`; `pgx.ErrNoRows` if there's no such location |
| `CreateSpace(ctx, {...})` | The new space's id. Takes every `spaces` column but `id`, timestamps included (2.4b) |
| `CreateOpeningWindow(ctx, {...})` | Nothing; inserts one `opening_windows` row |
| `GetSpace(ctx, id)` | The space as a `tenantdb.Space` (`sqlc.embed`) plus its location's `timezone`; `pgx.ErrNoRows` if there's no such space |
| `ListSpaces(ctx)` | Every space, by `id` |
| `ListOpeningWindows(ctx, spaceID)` | One space's windows, by weekday and opening time |
| `ListAllOpeningWindows(ctx)` | Every window, by space, weekday and opening time |
| `CreateBooking(ctx, {...})` | The new booking's id. Takes every `bookings` column but `id`; a NULL column is an invalid `pgtype` value (2.5) |
| `ExpireLapsedPending(ctx, {Now, SpaceID, StartsAt, EndsAt})` | Rows changed. Cancels at `Now` the space's bookings not cancelled or approved, with `approval_deadline <= Now`, overlapping `[StartsAt, EndsAt)` |
| `GetBooking(ctx, id)` | The row as a `tenantdb.Booking`; `pgx.ErrNoRows` if there's no such booking |
| `ApproveBooking(ctx, {ID, ApprovedAt, UpdatedAt})` | Rows changed, 1 or 0. Sets `approved_at` only if not cancelled, not approved, and `approval_deadline > ApprovedAt` |
| `CancelBooking(ctx, {ID, CancelledAt, UpdatedAt})` | Rows changed, 1 or 0. Sets `cancelled_at` only if not cancelled yet (2.6) |
| `ListMemberBookingsEndingAfter(ctx, {MemberID, After})` | The member's bookings with `cancelled_at IS NULL` and `ends_at > After`, by start then `id` (2.6) |
| `CancelMemberBookingsEndingAfter(ctx, {MemberID, Now})` | Rows changed. Cancels at `Now` every booking of the member not cancelled and with `ends_at > Now` (2.6) |
| `GetMember(ctx, id)` | The row as a `tenantdb.Member`, deleted or not; `pgx.ErrNoRows` if there's no such member (2.6) |
| `LockMember(ctx, id)` | The member's `deleted_at`, read `FOR SHARE`: the row stays locked until the transaction ends; `pgx.ErrNoRows` if there's no such member (2.6) |
| `DeleteMember(ctx, {ID, DeletedAt, UpdatedAt})` | Rows changed, 1 or 0. Sets `deleted_at` only if not deleted yet (2.6) |

## Tenant migrations

`migrations/tenant/`, run in each `tenant_<id>`. Table names are unqualified.

| Version | Creates |
| --- | --- |
| `00001_tenant.sql` | `locations`, `spaces`, `opening_windows`, `members` |
| `00002_bookings.sql` | `bookings` |

### `locations`

| Column | Type | Rule |
| --- | --- | --- |
| `id` | `bigint` | Identity, `GENERATED ALWAYS` |
| `name` | `text` | Not blank (`locations_name_not_blank`) |
| `timezone` | `text` | Not blank (`locations_timezone_not_blank`). An IANA name; validity checked in Go |
| `created_at`, `updated_at` | `timestamptz` | Default `now()` |

### `spaces`

| Column | Type | Rule |
| --- | --- | --- |
| `id` | `bigint` | Identity, `GENERATED ALWAYS` |
| `location_id` | `bigint` | References `locations` (`spaces_location_id_fkey`); a location with spaces can't be deleted |
| `name` | `text` | Not blank (`spaces_name_not_blank`) |
| `kind` | `text` | `hot_desk`, `private_office`, `meeting_room` or `phone_booth` (`spaces_kind_valid`) |
| `capacity` | `integer` | At least 1 (`spaces_capacity_positive`) |
| `max_booking_length` | `interval` | More than 0, at most 24 hours (`spaces_max_booking_length_valid`) |
| `available` | `boolean` | Default `true` |
| `created_at`, `updated_at` | `timestamptz` | Default `now()` |

### `opening_windows`

One open period on one weekday, the rows of a `WeeklyHours`. Primary key
`(space_id, weekday, opens_min)`. That windows on one day don't overlap or touch is checked by
`NewWeeklyHours`, not here.

| Column | Type | Rule |
| --- | --- | --- |
| `space_id` | `bigint` | References `spaces`, `ON DELETE CASCADE` |
| `weekday` | `smallint` | 0-6, Sunday 0, as `time.Weekday` (`opening_windows_weekday_valid`) |
| `opens_min` | `smallint` | Minutes since midnight, 0-1440 (`opening_windows_opens_valid`) |
| `closes_min` | `smallint` | 0-1440 (`opening_windows_closes_valid`), after `opens_min` (`opening_windows_opens_before_closes`) |

### `members`

| Column | Type | Rule |
| --- | --- | --- |
| `id` | `bigint` | Identity, `GENERATED ALWAYS` |
| `name` | `text` | Not blank (`members_name_not_blank`) |
| `email` | `text` | Lowercased and trimmed (`members_email_normalized`); unique among members not deleted (`members_email_key`, a partial unique index) |
| `created_at`, `updated_at` | `timestamptz` | Default `now()` |
| `deleted_at` | `timestamptz` | `NULL` until the member is deleted |

A booking insert and a member deletion meet on the member's row (2.6). `CreateBooking` reads it
`FOR SHARE` before inserting; `DeleteMember`'s `UPDATE` takes a `FOR NO KEY UPDATE` lock, which
conflicts with that, so whichever comes second waits for the first to commit. The foreign key
check on `bookings.member_id` takes only `FOR KEY SHARE`, which an `UPDATE` of a non-key column
doesn't conflict with, so the foreign key alone doesn't make them wait.

### `bookings`

One booking of a space for a member. The rows of a `Booking`; a `NULL` here is a zero value there.

| Column | Type | Rule |
| --- | --- | --- |
| `id` | `bigint` | Identity, `GENERATED ALWAYS` |
| `space_id` | `bigint` | References `spaces` (`bookings_space_id_fkey`); a space with bookings can't be deleted |
| `member_id` | `bigint` | References `members` (`bookings_member_id_fkey`); a member with bookings can't be deleted, only soft-deleted. Indexed by `bookings_member_id_idx` |
| `booked_by` | `bigint` | Staff ID, `NULL` when the member booked. No foreign key until the staff table (M4) |
| `attendees` | `integer` | At least 1 (`bookings_attendees_positive`); capacity is checked by `NewBooking` |
| `starts_at`, `ends_at` | `timestamptz` | `starts_at < ends_at` (`bookings_starts_before_ends`); `ends_at` is not included |
| `approval_deadline` | `timestamptz` | Set exactly when `booked_by` is (`bookings_approval_iff_staff`) |
| `approved_at` | `timestamptz` | `NULL` until approved; only with a deadline (`bookings_approved_needs_deadline`) |
| `cancelled_at` | `timestamptz` | `NULL` until cancelled or declined |
| `created_at`, `updated_at` | `timestamptz` | Default `now()` |

`bookings_no_overlap` is an exclusion constraint, `EXCLUDE USING gist (space_id WITH =,
tstzrange(starts_at, ends_at) WITH &&) WHERE (cancelled_at IS NULL)`: no two bookings of one space
overlap unless one is cancelled. Pending bookings count, even past their deadline. A refused
insert has SQLSTATE `23P01`, which `postgres.BookingStore` turns into `ErrSlotTaken`. A lapsed
pending booking keeps its slot until a new booking needs it: `ExpireLapsedPending` cancels it in
the new booking's transaction (2.5).
