# Database

Postgres 18, one database for every operator, one schema per operator. So far only
`cowbook create-tenant` connects to it; 2.4 adds the Postgres stores.

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
| `make migrate-check` | Up, reset, up: proves every Down migration works |
| `make test-integration` | Tests tagged `integration`, against `DATABASE_URL` |
| `make create-tenant SLUG=... NAME=...` | Runs `cowbook create-tenant` with `.env` loaded |

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

## Tenant migrations

`migrations/tenant/`, run in each `tenant_<id>`. Table names are unqualified.

| Version | Creates |
| --- | --- |
| `00001_tenant.sql` | `locations`, `spaces`, `opening_windows`, `members` |

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
