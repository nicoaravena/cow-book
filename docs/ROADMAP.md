# Roadmap

This project exists mainly to **learn Go**. The vehicle is the multi-tenant coworking booking platform described in `README.md`.

**Finish line:** every README rule works through an HTTP API plus simple web pages, running locally.

**Next up:** step 1.2.

## How to work

- Agent writes one small step at a time, about 50-200 lines plus tests.
- Each step delivers the code, its tests, the commands to run, and a short note on the Go ideas it introduces.
- **Each step ends with a stop for review.** The user reads and approves it before the next step starts.
- The user commits, or Agent commits when asked. Per the global `AGENTS.md`, commits carry **no** `Co-Authored-By` line for the agent.

### Checks

- Every step: `make test` and `make lint` (`go vet`, `gofmt -l`, `staticcheck`).
- From M2: `make test-integration`.
- From M3: exercise the running server with `curl`, with commands given in each step.
- At the end: the 7.7 run-through in a browser, and a checklist mapping each README rule to the test that proves it.

`make lint` is expected to fail on unused `Booking` and `Member` fields until step 1.4 is done.

## Steps

### M0: Foundation

- [x] **0.1** Git repo, `.gitignore`, Makefile (`test`, `lint`), replace the broken tests.

### M1: Domain rules, no database

- [x] **1.1** `NewSpace` and validation. *Go: exported vs unexported, constructors, sentinel errors, table-driven tests.*
- [ ] **1.2** Weekly opening hours and `Contains`, with timezone and DST tests. *Go: `time`, value types, methods.*
- [ ] **1.3** `NewBooking`: all booking rules. *Go: `errors.Is`, wrapping with `%w`.*
- [ ] **1.4** `NewMember`, Actor (member, front desk with locations, admin) and cancel rule, `Cancel`. *Go: pointer vs value receivers, small enums with `iota`.*
- [ ] **1.5** `BookingStore` interface and `BookingService` with injected clock. *Go: implicit interfaces, `context.Context`.*
- [ ] **1.6** In-memory store plus a concurrent booking test. *Go: goroutines, `sync.Mutex`, `sync.WaitGroup`, `-race`.*

### M2: Postgres

- [ ] **2.1** `compose.yaml`, goose, platform migration: `tenants` and the `shared` schema. *Go: `go tool` directive.*
- [ ] **2.2** Tenant migration (locations, spaces, members) and a provisioning function that creates `tenant_<id>` and migrates it, with cleanup on failure. Must confirm goose works per schema (see [Database roles](#database-roles)). *Go: `embed`, `database/sql` vs pgx, `pgx.Identifier`.*
- [ ] **2.3** Bookings tenant migration with the exclusion constraint, shown working with raw SQL.
- [ ] **2.4** sqlc setup, pgx pool, `WithTenant` transaction wrapper, space store. A test proves `search_path` doesn't carry over on a reused connection. *Go: closures for transaction scope, generated code, build tags.*
- [ ] **2.5** Booking store: `23P01` maps to `ErrSlotTaken`. Same contract tests as the in-memory store, then many goroutines racing for one slot. *Go: `errors.As`, subtests, shared test helpers.*
- [ ] **2.6** Cancel in a transaction; the slot is bookable again. *Go: `defer` with rollback.*

### M3: HTTP API

- [ ] **3.1** `cmd/cowbook`, config from env, `slog`, `/healthz`, graceful shutdown. *Go: `signal.NotifyContext`, `http.Server`.*
- [ ] **3.2** `cowbook migrate` and `cowbook create-tenant` subcommands for local use. *Go: `flag`, subcommands, exit codes.*
- [ ] **3.3** chi, tenant middleware, `GET spaces`, JSON helpers. *Go: handlers, middleware as function wrapping, `httptest`.*
- [ ] **3.4** Availability for a space and date.
- [ ] **3.5** `POST bookings` with error-to-status mapping (409, 422). Uses a **temporary** `X-Member-ID` header, removed in 4.3.
- [ ] **3.6** Cancel, my bookings, the day's bookings, booking history.
- [ ] **3.7** Management endpoints for locations, spaces and members.

### M4: Login

- [ ] **4.1** Credentials migration and password hashing.
- [ ] **4.2** Sessions: log in, log out, token hashing. *Go: `crypto/rand`, cookies.*
- [ ] **4.3** Auth middleware, actor in request context, remove the temporary header. *Go: context keys.*
- [ ] **4.4** Member self-registration: public endpoint with a basic per-IP limit.
- [ ] **4.5** Staff accounts: admins add and remove staff and assign front desk locations (`staff_locations` migration).
- [ ] **4.6** Role and location checks on every route, tested as a table of route by role.
- [ ] **4.7** Temporary password reset with forced change at next login.

### M5: Tenant isolation and fairness

- [ ] **5.1** Database roles: per-tenant roles, platform, provisioner, app locked down. `WithTenant` adds `SET LOCAL ROLE`, and provisioning creates the tenant role.
- [ ] **5.2** Isolation suite: two tenants, every endpoint, plus direct SQL naming the other tenant's schema, which must be refused.
- [ ] **5.3** Self-service operator signup using the provisioner pool.
- [ ] **5.4** Plans, per-operator rate limits, query timeouts, suspended operators blocked. *Go: `sync.Map` or mutex-guarded maps, `x/time/rate`.*
- [ ] **5.5** Migrating all tenants safely: `cowbook migrate` reports each tenant, carries on after a failure, and can be re-run. *Go: bounded concurrency with worker goroutines.*

### M6: Platform support

- [ ] **6.1** Support users, `cowbook create-support-user` CLI (password from stdin, exit codes).
- [ ] **6.2** Support login, paginated list of operators with staff counts (one query per listed schema), plan and status, audit log mechanism, operator details view (audited).
- [ ] **6.3** Suspend and reactivate, change plan, both audited.
- [ ] **6.4** Manage operator admin accounts, including temporary password reset, audited.
- [ ] **6.5** Test proving the support role cannot read members or bookings at the database level.

### M7: Web pages

- [ ] **7.1** Layout, embedded templates, CSS, login page. *Go: `embed`, `html/template` escaping.*
- [ ] **7.2** Member: availability and booking form.
- [ ] **7.3** Member: my bookings and cancel.
- [ ] **7.4** Front desk: the day's bookings at a glance.
- [ ] **7.5** Staff: manage locations, spaces, members and staff accounts.
- [ ] **7.6** Operator signup, member registration and support pages.
- [ ] **7.7** Run-through documented in the README: operator signs up, adds a location and space, member registers and books, sees a conflict, cancels, support suspends the operator.

Milestones can be reordered or dropped between steps.

## Scope

**Goals**

- Every README rule is enforced and proven by a test.
- The user understands every line, so each step is small and explained.

**Non-goals**

- Real billing or payments, cloud deployment, dedicated capacity or per-tenant databases (written up as a future option only).
- Metrics dashboards and per-customer cost accounting. The support view shows simple per-operator counts only.
- Custom integrations, email delivery, password reset by email.

**Anti-goals (failure even if it works)**

- Large steps the user can't review, or generated code that isn't explained.
- Tenant isolation that relies only on `search_path` being set correctly.
- Overlap prevention that relies only on a check in application code.

## Decisions

Decided by the user:

- **Stack direction:** popular libraries plus Postgres.
- **Tenancy:** one database with one Postgres schema per tenant.
- **Member accounts:** members self-register on their operator's page. Locked-out members get a new password from staff.
- **Staff roles:** two operator roles, admin and front desk. Admins manage staff accounts and cover the whole operator. Front desk staff are limited to their assigned locations.
- **Support view:** the operator list shows staff counts, plan and status only. No member or booking totals, so the database guarantee stays absolute.

Proposed by Agent and accepted in conversation:

- **Libraries:** chi (router), pgx (driver), sqlc (generated queries), goose (migrations), `html/template` (pages, no JS framework).
- **Double bookings:** a Postgres exclusion constraint decides the winner.
- **Login:** email and password, server-side sessions. Roles: member, front desk, admin, platform support.

Agent's calls, open to rejection:

- **Second lock for schemas:** each tenant also gets its own database role, with access to its own schema only. `search_path` alone would let one bug read another tenant.
- **Front desk scope:** spaces, bookings and cancellations are limited to assigned locations. Members belong to the whole operator, so any staff can look them up. This narrows the README's "the operator's staff" for cancelling, as a consequence of location limits.
- **Locked-out accounts:** staff reset members, admins reset staff, support resets admins. A reset produces a single-use temporary password shown once, which must be changed at next login. Support resets are audited.
- **Support "configuration":** plan, status and admin accounts. Spaces, opening hours and locations stay with the operator.
- **Support "investigate problems":** operator status, plan, staff counts and that operator's audit history.
- **Audit scope:** every support change, and every time support opens one operator's details. Listing operators is not recorded.
- **Suspension:** a suspended operator's routes all return 403, for members and staff. Data is kept and support can reactivate.
- **Plans:** three fixed plans in code, each with a request rate and a maximum number of spaces.
- **Availability:** booked intervals and opening hours, to the minute. No fixed slot grid.

## How each README rule is enforced

| README rule | Enforced by |
|---|---|
| No overlapping bookings; only one simultaneous winner | Exclusion constraint in each tenant schema on `(space_id, tstzrange(starts_at, ends_at))` where not cancelled; Postgres error `23P01` maps to `ErrSlotTaken` |
| End after start, within opening hours, capacity, max length | Domain constructor `cowbook.NewBooking` |
| Only the booker or staff can cancel | Domain rule, checked in the service: the booker, an admin, or front desk assigned to that location |
| Cancelling frees the slot immediately | `cancelled_at` set in one transaction; the constraint ignores cancelled rows; history is kept |
| No operator sees another's data | A schema per tenant, plus a per-tenant DB role that has no access to other schemas; other tenants' resources return 404 |
| Support can manage accounts, not member data or bookings | Separate DB role granted only the `staff` table in each tenant schema, so the database refuses even if the code is wrong |
| Every support action recorded | Audit row written in the same transaction as the action; role can only insert and read |
| One customer must not hurt others | Per-operator rate limit by plan (`golang.org/x/time/rate`), query timeouts |
| Fast onboarding | Self-service signup: tenant row marked `provisioning`, schema and role created, migrations and first admin applied, then marked `active`. On failure the schema and role are dropped |

## Architecture

Layering: delivery (`httpapi`, `web`, `main`) -> `cowbook` services -> `cowbook` domain; `postgres` implements interfaces defined in `cowbook`. The domain imports no database or HTTP packages. The clock is injected.

```
cmd/cowbook/main.go      composition root, CLI subcommands
cowbook/                 domain + services
internal/postgres/       store implementations, sqlc output
internal/httpapi/        JSON API at /api/o/{slug}/...
internal/web/            HTML pages at /o/{slug}/..., embedded templates
migrations/platform/     goose SQL, run once: tenants, support users, audit log
migrations/tenant/       goose SQL, run in every tenant schema
compose.yaml, Makefile, sqlc.yaml
```

### Domain model

Agent's calls, open to rejection at each step.

- `Tenant`: slug, name, plan, status (provisioning, active or suspended). The tenant comes from the URL slug. Its schema name is `tenant_<id>`, built from the numeric ID and never from the slug or other user input.
- `Location`: name, IANA timezone. Added for chains with many locations.
- `Space`: location, name, kind, capacity, max booking length, weekly opening hours, available flag.
- `Booking`: space, member, attendees (new, needed for the capacity rule), start, end, `cancelledAt`. A booking must fit inside one opening window, so it cannot cross midnight.
- `Member` and `Staff`: **separate tables**, so support can be granted access to staff accounts and denied member data. Email is unique per tenant. `Staff` has a role (admin or front desk); a `staff_locations` table holds front desk assignments.
- `SupportUser`, `AuditEntry`: platform level, no tenant.

Type conventions, still to be applied to `Booking` and `Member` in `cowbook/types.go`:

- IDs are `int64`, matching Postgres `bigint` and sqlc.
- Booking `deletedAt` becomes `cancelledAt`.
- Fields stay unexported and are set through validating constructors.

### Database roles

- `cowbook_app`: tenant traffic. Can read `tenants` to resolve a slug, and has no table access of its own otherwise. It may switch to any tenant role.
- `cowbook_t_<id>`: one per tenant, access to that tenant's schema only. Every tenant transaction starts with `SET LOCAL ROLE` and a local `search_path` of that schema plus `shared`, both of which end with the transaction.
- `cowbook_platform`: support. Grants on `tenants`, audit, and the `staff` table in each tenant schema only.
- `cowbook_provisioner`: signup and tenant migrations. Can create schemas and roles.
- `cowbook_owner`: platform migrations only.

The app holds three connection pools: app, platform and provisioner. The `shared` schema holds the `btree_gist` extension that the exclusion constraint needs. With no tenant selected, tenant tables aren't visible and access is refused, so a forgotten wrapper fails closed.

sqlc queries use unqualified table names, generated from `migrations/tenant/`. goose runs per tenant through a `database/sql` connection with `search_path` set in its connection settings.

### Security

- Passwords hashed with bcrypt (`golang.org/x/crypto`).
- Session cookies are `HttpOnly`, `SameSite=Lax`; only a SHA-256 hash of the token is stored.
- Stdlib `http.CrossOriginProtection` guards against CSRF.
- `Secure` cookie flag configurable, off for local HTTP.
- Secrets live in a gitignored `.env`; `.env.example` lists the variable names.

### Testing

- `go test ./...`: fast unit tests, no database.
- `make test-integration`: `-tags integration` against Postgres in compose, using `-race`. Includes the concurrent-booking test and the isolation suite.
- CLI exit codes: `0` ok, `1` general error, `2` usage error.

### Dependencies

Added at the step that first needs them, for approval then:

`go-chi/chi/v5`, `jackc/pgx/v5`, `x/crypto`, `x/time`, and as `go tool` entries `sqlc` and `goose`. Image: `postgres:18`.

Already added: `staticcheck`, as a `go tool` entry.

## Risks and trade-offs

- **Schema per tenant, benefits:** strong separation that's easy to see, and one operator can be exported, restored or moved to a dedicated database on its own.
- **Schema per tenant, costs at scale:** every operator adds its own tables, indexes and role, so thousands of operators slow backups and catalog lookups and weigh on the README's "economical as operators grow". This won't show up locally. A free tier with many small operators is the case to watch.
- **Migrations across tenants:** a change runs once per schema, so it takes longer, can fail partway, and leaves tenants on different versions for a while. Old code must keep working during a rollout. 5.5 handles reporting and re-runs.
- **Signup runs DDL:** a public request ends up creating schemas and roles, so signup needs its own strict rate limit and a separate, more privileged pool.
- **Reused connections:** a mistake here could expose the wrong tenant, so settings are transaction-local and tested in 2.4. pgx's prepared statement cache can raise errors while schemas are on different versions; 2.4 checks this and picks a pgx query mode.
- **Shared instance:** one operator's heavy use can still affect others beyond what rate limits catch, because schemas don't separate CPU or disk. Dedicated databases remain a documented future option.
- **In-memory rate limiter:** works for one process only; multiple instances would need a shared store. Acceptable since this runs locally.
- **Limited support view:** with no member or booking totals, support can't tell how busy an operator is, and the platform can't estimate cost per operator from this view. That part of the README stays open.
- **Public registration:** anyone can create a member account on an operator's page, with no email check. The per-IP limit slows abuse but doesn't verify identity.
- **Per-tenant roles arrive in M5:** schemas and `WithTenant` exist from M2, so nothing needs reshaping, but the second lock and the isolation proof come later. M3 and M4 run only locally.
