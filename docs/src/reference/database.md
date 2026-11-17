# Database

Postgres 18, one database for every operator, one schema per operator. Nothing in the app
connects to it yet; 2.4 adds the Postgres stores.

## Running it locally

`compose.yaml` starts Postgres as the `db` service. Settings come from `.env`, which is gitignored;
`.env.example` lists the names.

| Variable | Used by | Means |
| --- | --- | --- |
| `POSTGRES_PASSWORD` | compose | Password of the `cowbook` user. Required |
| `POSTGRES_PORT` | compose | Host port, default `5432` |
| `DATABASE_URL` | goose, later the app | Connection string with the same password |

| Make target | Does |
| --- | --- |
| `make db-up` | Start Postgres and wait until it is healthy |
| `make db-down` | Stop it; data stays in the `pgdata` volume |
| `make db-psql` | Open `psql` inside the container |
| `make migrate-platform` | Apply `migrations/platform` and print the status |
| `make migrate-check` | Up, reset, up: proves every Down migration works |

## Schemas

| Schema | Holds |
| --- | --- |
| `platform` | Data about operators |
| `shared` | The `btree_gist` extension, for exclusion constraints |
| `tenant_<id>` | One operator's data (2.2) |
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
