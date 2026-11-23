# Roadmap

This project exists mainly to **learn Go**. The vehicle is the multi-tenant coworking booking platform described in `README.md`.

**Finish line:** every README rule works through an HTTP API plus simple web pages, running locally with `cowbook serve`.

**Next up:** step 3.2.

## How to work

- Agent writes one small step at a time, about 50-200 lines plus tests.
- Each step delivers the code, its tests, the commands to run, and a short note on the Go ideas it introduces.
- Each step also delivers its concepts page in `docs/src/concepts/`, adds it to `docs/src/SUMMARY.md` and the topic index, and updates the reference pages in `docs/src/reference/` that its code touches.
- Each step also delivers a **runnable result** in the one binary, `cmd/cowbook`, which grows every step and is the full app at the last one. Usually that is new or changed commands in `cowbook repl` (`go run ./cmd/cowbook repl`), later new subcommands such as `serve` or `migrate`. The step note lists the commands to type. Steps the binary can't reach yet say what to run instead.
- **Each step ends with a stop for review.** The user reads and approves it before the next step starts.
- The user commits, or Agent commits when asked. Per the global `AGENTS.md`, commits carry **no** `Co-Authored-By` line for the agent.

### Checks

- Every step: `just test` and `just lint` (`go vet`, `gofmt -l`, `staticcheck`), and `just docs` builds with no warnings.
- Every step from 1.9: its runnable result works through `cmd/cowbook`.
- From M2: `just migrate-check`, and `just test-integration` from 2.2, where the first integration test arrives.
- From M3: exercise the running server with `curl`, with commands given in each step.
- At the end: the 7.7 run-through in a browser, and a checklist mapping each README rule to the test that proves it.

## Steps

### M0: Foundation

- [x] **0.1** Git repo, `.gitignore`, justfile (`test`, `lint`), replace the broken tests.

### M1: Domain rules, no database

- [x] **1.1** `NewSpace` and validation. *Go: exported vs unexported, constructors, sentinel errors, table-driven tests.*
- [x] **1.2** Weekly opening hours and `Contains`, with timezone and DST tests. *Go: `time`, value types, methods.*
- [x] **1.3** `NewBooking`: all booking rules. *Go: `errors.Is`, wrapping with `%w`.*
- [x] **1.4** `NewMember`, Actor (member, front desk with locations, admin) and cancel rule, `Cancel`. *Go: pointer vs value receivers, small enums with `iota`.*
- [x] **1.5** `BookingStore` interface and `BookingService` with injected clock. *Go: implicit interfaces, `context.Context`.* `Book` takes no `Actor` here; 1.6 adds it.
- [x] **1.6** Who can book, and staff bookings awaiting approval. `Book(ctx, actor, spaceID, p)`: a member books only for themselves, front desk for any member at their assigned locations, an admin for any member. A staff-made booking starts pending, with an approval deadline 24 hours after creation or at the booking's start, whichever comes first. `Booking.Approve(actor, now)` lets only that member approve, before the deadline and while not cancelled; the service gets `Approve`. Declining is the existing `Cancel`. *Go: state derived from zero-value timestamps, unexported helper methods.*
- [x] **1.7** Deleting a member. `MemberService.Delete` marks the member deleted and cancels every booking of theirs that hasn't ended, pending ones included, through an unexported domain method that skips the actor rules (the system cancels, not a person). `Book`, `Cancel` and `Approve` refuse a deleted member acting as a member. One store method saves the deletion and the cancellations together, so Postgres can use one transaction. *Go: slices of values vs pointers, changing elements in a loop.*
- [x] **1.8** In-memory store plus a concurrent booking test, also covering `Approve`, member deletion and a pending booking holding its slot. *Go: goroutines, `sync.Mutex`, `sync.WaitGroup`, `-race`.*
- [x] **1.9** `cmd/cowbook`, the app's one binary, with its first subcommand `repl` over the in-memory store, with a settable clock, `space add`, `member add`, acting as a member or staff (`as`), `book`, `help`, and a script mode that echoes each piped line. Errors print and the session carries on. *Go: `os.Args` subcommands, `bufio.Scanner`, `io.Reader`/`io.Writer` injection, `strings.Fields`, `time.Parse`, a map of command functions.*
- [x] **1.10** REPL catches up: `cancel`, `approve`, `member delete`, listings of spaces, members and bookings with their state. *Go: `text/tabwriter`.*

### M2: Postgres

Runnable: 2.1-2.3 give a SQL script for `psql` against the compose database (for example two overlapping inserts, the second refused). 2.4-2.6 are run through their integration tests. From 2.7 `cowbook repl` takes `-store=postgres`, so data survives a restart.

- [x] **2.1** `compose.yaml`, goose, platform migration: `tenants` and the `shared` schema. *Go: `go tool` directive.*
- [x] **2.2** Tenant migration (locations, spaces, members) and a provisioning function that creates `tenant_<id>` and migrates it, with cleanup on failure. Must confirm goose works per schema (see [Database roles](#database-roles)). *Go: `embed`, `database/sql` vs pgx, `pgx.Identifier`.* Brought `cowbook create-tenant <slug> <name>` forward from 3.2 as the runnable result.
- [x] **2.3** Bookings tenant migration with the exclusion constraint, shown working with raw SQL. Also stores who made the booking, the approval deadline and the approval time. The constraint ignores only cancelled rows, so pending bookings hold their slot. Added `just migrate-tenant TENANT=tenant_<id>` so existing tenants get the new version.
- [x] **2.4** sqlc setup, pgx pool with query mode `describe_exec`, `WithTenant` transaction wrapper. Tests prove `search_path` doesn't carry over on a reused connection (after a commit, an error and a panic), and show why the default `cache_statement` mode fails across tenants on different schema versions. Split from the space store, now 2.4b. *Go: closures for transaction scope, `defer` with rollback, generated code, build tags.*
- [x] **2.4b** Space store: a `cowbook` function that rebuilds a `Space` from stored values, and accessors for its location, hours, availability and timestamps. `GetSpace` with its location's timezone and opening windows, plus create and list, all through `WithTenant`. Integration tests. *Go: mapping rows to domain types.*
- [x] **2.5** Booking store: `23P01` maps to `ErrSlotTaken`. Same contract tests as the in-memory store, then many goroutines racing for one slot. Lazy expiry: in the same transaction, `CreateBooking` first cancels lapsed pending bookings that overlap the new one, because a constraint can't use `now()`; approving checks the deadline the same way. *Go: `errors.As`, subtests, shared test helpers.* Has `CreateBooking`, `GetBooking` and `SaveApproval`; `SaveCancellation` and `ListMemberBookingsEndingAfter` move to 2.6.
- [x] **2.6** Cancel in a transaction (`SaveCancellation`, and `ListMemberBookingsEndingAfter` for deletion), so `postgres.BookingStore` meets `cowbook.BookingStore`; the slot is bookable again. Member deletion in one transaction: mark the member deleted and cancel their bookings. A booking insert refuses a member deleted at the same moment (lock the member row, or insert only while the member isn't deleted), proven by a test. *Go: transactions spanning several queries.* Done with a member store holding `GetMember` and `SaveDeletion`, and `RestoreMember`; the member row's lock is what both sides wait on.
- [x] **2.7** Creating and listing members in Postgres (2.6 brings `GetMember` and `SaveDeletion`), and `cowbook repl -store=postgres -tenant=<slug>`: the REPL over the three Postgres stores, so data survives a restart. The slug is looked up in `platform.tenants`. The REPL's session stops holding `*memstore.Store` and takes a small interface both stores meet. *Go: interfaces defined by the consumer.* Also brought: `ListBookings` in Postgres, a location per timezone (`LocationFor`), `postgres.Stores` by struct embedding, `ErrEmailTaken`, and `flag.NewFlagSet` for `repl` only.

### M3: HTTP API

Runnable: 3.1-3.2 add the `serve`, `migrate` and `create-tenant` subcommands, the server exercised with `curl`. From 3.3 `cowbook repl` takes `-server http://localhost:8080` and talks to the running server. Its commands go through a small `Backend` interface with an in-process and an HTTP implementation, and each new endpoint gets a REPL command. Whether in-process mode stays is decided at 3.3.

- [x] **3.1** `cowbook serve`: config from env, `slog`, `/healthz`, graceful shutdown. *Go: `signal.NotifyContext`, `http.Server`.* `/healthz` pings Postgres (decided by the user). Routes in `internal/httpapi` on the stdlib `ServeMux` until chi in 3.3; the run loop in `internal/server`; `just serve`.
- [ ] **3.2** `cowbook migrate` subcommand for local use, and `create-tenant` (added in 2.2) moves with it; dispatch moves from a `switch` on `os.Args` to `flag`. *Go: `flag`, `flag.NewFlagSet` per subcommand, exit codes.*
- [ ] **3.3** chi, tenant middleware, `GET spaces`, JSON helpers. *Go: handlers, middleware as function wrapping, `httptest`.*
- [ ] **3.4** Availability for a space and date.
- [ ] **3.5** `POST bookings` with error-to-status mapping (409, 422). Uses a **temporary** `X-Member-ID` header, removed in 4.3, which builds a member `Actor`. Staff booking for a member waits for staff login.
- [ ] **3.6** Cancel, approve, my bookings (pending ones with their deadline), the day's bookings, booking history.
- [ ] **3.7** Management endpoints for locations, spaces and members. Deleting a member goes through `MemberService.Delete`, which cancels their bookings.

### M4: Login

Runnable: `cowbook repl` gets `login`, which replaces `as` and holds the session cookie.

- [ ] **4.1** Credentials migration and password hashing.
- [ ] **4.2** Sessions: log in, log out, token hashing. *Go: `crypto/rand`, cookies.*
- [ ] **4.3** Auth middleware, actor in request context, remove the temporary header. *Go: context keys.*
- [ ] **4.4** Member self-registration: public endpoint with a basic per-IP limit.
- [ ] **4.5** Staff accounts: admins add and remove staff and assign front desk locations (`staff_locations` migration).
- [ ] **4.6** Role and location checks on every route, tested as a table of route by role, including staff booking for a member and member-only approval.
- [ ] **4.7** Temporary password reset with forced change at next login.

### M5: Tenant isolation and fairness

Runnable: REPL commands for operator signup and switching tenants. M6 adds support commands.

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

Runnable: the browser, against `cowbook serve`. `cowbook repl` keeps up only where it's cheap.

- [ ] **7.1** Layout, embedded templates, CSS, login page. *Go: `embed`, `html/template` escaping.*
- [ ] **7.2** Member: availability and booking form.
- [ ] **7.3** Member: my bookings and cancel, and approve or decline bookings staff made for them.
- [ ] **7.4** Front desk: the day's bookings at a glance, with pending ones marked, and booking for a member.
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
- **Member deletion:** members are soft-deleted. The record stays, so bookings and history still point at a real member.
- **Booking for a member:** staff can book for a member, and the member must approve it. Until then the booking is pending and holds the slot. If the member doesn't approve by the approval deadline, it lapses and frees the slot.
- **Deleted members:** they can't cancel bookings. Deleting a member cancels their bookings automatically.
- **Opening hours storage (2.2):** a child table `opening_windows`, one row per window, rather than a `jsonb` column.
- **Email reuse (2.2):** a deleted member's email can register again.
- **`create-tenant` early (2.2):** brought forward from 3.2 as 2.2's runnable result.
- **2.4 split (2.4):** the plumbing (sqlc, pool, `WithTenant`) stays 2.4, the space store becomes its own step, so each stays reviewable.
- **REPL on Postgres at M2's end (2.4):** `cowbook repl -store=postgres` waits for 2.7, when spaces, members and bookings are all in Postgres, rather than mixing stores.
- **A location per timezone (2.7):** `space add ... [timezone]` keeps its syntax; both stores get `LocationFor(ctx, tz)`, which finds the tenant's location in that zone or adds one named after it. Rejected: a `location add` command (part of 3.7 brought forward, and a syntax change) and Postgres-only location 1 (the two modes would differ). 3.7 replaces it.
- **Fixed REPL clock in both modes (2.7):** Postgres mode also starts at Mon 2026-11-16 08:00 UTC, so the modes behave the same. Rejected: the wall clock in Postgres mode.
- **Contract tests ported, not shared (2.5):** memstore's booking tests are copied into `internal/postgres` with the same names, rather than moved into a shared suite package both stores run. Cost: two copies that can drift.
- **No lazy expiry in memstore (2.5):** the in-memory store keeps a lapsed booking's slot held; only Postgres expires it.

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
- **`Cancel` takes the `Space`:** a `Booking` holds only `spaceID`, so the location has to come from somewhere. Passing the `Space` matches `NewBooking` and lets a mismatched space be refused, where a wrong location ID would silently authorize against the wrong location. Cost: 2.6 has to load the space to cancel.
- **Ended bookings can't be cancelled:** not a README rule. Cancelling last Tuesday's booking would rewrite who used the room. A booking in progress can still be cancelled, which frees the rest of the slot.
- **Three small store interfaces:** `BookingStore`, `SpaceStore` and `MemberStore`, because 2.4 already plans a separate space store. Cost: 1.6 implements all three.
- **`GetSpace` returns a `*time.Location`:** `NewBooking` only needs the timezone. The `Location` type arrives with location management in 3.7.
- **Approval deadline:** 24 hours after a staff booking is made, or the booking's start if that comes first. A constant in code; it could vary by plan later.
- **Every staff booking needs approval,** admin-made ones included.
- **Front desk book only at their locations,** matching the cancel rule.
- **Declining is cancelling:** the member declines a pending booking with `Cancel`, with no separate state.
- **Deletion cancels bookings in progress too,** which frees the rest of the slot. Ended bookings stay as they are, to keep the history true.
- **Lazy expiry:** lapsed pending bookings are cancelled when a new booking needs their slot, not by a background job.
- **Deleted members are refused first:** `BookingService.Cancel` and `Approve` look up a member actor before anything else and return `ErrMemberDeleted`. It's about the actor themselves, so it leaks nothing, and it reads better than `ErrAlreadyCancelled`. Staff still cancel a deleted member's bookings. Cost: one more store read per member request.
- **`SaveDeletion` lives on `MemberStore`:** no fourth interface. Cost: whatever implements `MemberStore` for `BookingService` must implement it too.
- **`MemberService.Delete` takes no actor yet:** who may delete arrives with member management (3.7) and role checks (4.6).
- **Bookings in a new migration (2.3):** `00002_bookings.sql`, not an edit of the applied `00001`.
- **`booked_by` without a foreign key (2.3):** the staff table arrives in M4, which adds it.
- **Booking references don't cascade (2.3):** a space or member with bookings can't be deleted, so history stays. Members are soft-deleted.
- **Approval columns checked in the table (2.3):** `booked_by` and `approval_deadline` are set together, and `approved_at` needs a deadline. Capacity, length and opening hours stay in `NewBooking`.
- **`just migrate-check` in a throwaway database (2.3):** its `reset` used to run in the dev database, where it would drop `platform.tenants` with every tenant row. From 2.3 tenant constraints depend on `btree_gist`, so it failed there instead. It now creates, checks and drops `cowbook_migrate_check`.
- **`just migrate-tenant` (2.3):** migrates one existing tenant with goose, `search_path` passed in the URL. A stopgap until 5.5 migrates every tenant.
- **Query mode `describe_exec` (2.4):** `NewPool` sets it, because pgx's default `cache_statement` fails with `cached plan must not change result type` when tenants on different migration versions share a connection, as during 5.5. Cost: two round trips per query. `exec` (one round trip, parameter types taken from Go values) is the fallback if latency matters.
- **`WithTenant` takes a closure (2.4):** `fn(q *tenantdb.Queries) error`, so callers never hold the transaction and can't forget to commit, roll back or set the path. Results leave through variables the closure captures.
- **`set_config('search_path', $1, true)` (2.4):** transaction-local like `SET LOCAL`, and the path is a parameter rather than SQL text.
- **Generated code committed (2.4):** `go build` needs neither sqlc nor cgo; `just lint` runs `sqlc diff` so it can't fall behind.
- **Step names 2.4b and 2.7 (2.4):** not renumbering 2.5 and 2.6 keeps about 20 forward references in earlier pages and comments correct.
- **`RestoreSpace` goes through `NewSpace` (2.4b):** a stored row is checked by the same rules as a new space, so a row changed by hand into something the domain refuses fails on read. Cost: if a rule tightens, old rows stop loading until a migration fixes them.
- **`SpaceRecord` embeds `SpaceParams` (2.4b):** the record is the params plus what a store keeps, rather than a long parameter list. The same shape should fit `Booking` in 2.5.
- **The store writes the domain's timestamps (2.4b):** not the column default `now()`, so the injected clock stays the one source of time. pgx stores them cut to the microsecond.
- **One `SpaceStore` per tenant (2.4b):** `NewSpaceStore(pool, tenantID)`, because `cowbook.SpaceStore.GetSpace` takes no tenant. It holds no connection, so M3 can build one per request.
- **Unknown location from the foreign key (2.4b):** `CreateSpace` maps `23503` on `spaces_location_id_fkey` to `ErrInvalidLocation` instead of reading the location first, which another request could delete in between.
- **`ListSpaces` in two queries (2.4b):** spaces, then every opening window grouped in a map, not one query per space. Its signature matches `memstore.ListSpaces` for 2.7.
- **Lazy expiry's clock is `b.CreatedAt()` (2.5):** `NewBooking` took it from the service's injected clock, so `CreateBooking` needs no `now` parameter and the interface is unchanged. A booking lapses at its deadline (`approval_deadline <= now`), matching `State` and `Approve`.
- **Expiry and insert in one transaction (2.5):** if the insert is refused, the expiry rolls back with it. Harmless: the next attempt expires the same rows. Only bookings overlapping the new one are expired.
- **`SaveApproval` checks in its `UPDATE` (2.5):** not cancelled, not approved, and `approval_deadline > approved_at`, so the check and the write are one statement. On 0 rows it reads the row in the same transaction to name the refusal, adding `ErrApprovalExpired` to the losers. It writes only `approved_at` and `updated_at`.
- **`RestoreBooking` skips `NewBooking` (2.5):** that needs the actor and space and refuses past starts. It checks member, space, attendees and end after start; the approval columns are left to the 2.3 checks. `BookingRecord` embeds `BookingParams`, as planned in 2.4b.
- **Foreign keys on bookings (2.5):** `23503` on `bookings_space_id_fkey` and `bookings_member_id_fkey` map to `ErrSpaceNotFound` and `ErrMemberNotFound`, for a space or member removed between the service's read and the insert.
- **`BookingStore` methods split over 2.5 and 2.6 (2.5):** cancel and member listing belong with 2.6's transactions; the compile-time interface check arrives then.
- **The store guards the cancel write:** cancelling is read, check, write, so two simultaneous cancels can both pass `Booking.Cancel`. `SaveCancellation` must give the loser `ErrAlreadyCancelled`. Real concurrency is tested in 1.8 and 2.6.
- **`SaveCancellation` checks in its `UPDATE` (2.6):** `cancelled_at IS NULL` in the `WHERE`, as `SaveApproval` does; on 0 rows a read in the same transaction tells not found from already cancelled. It writes only `cancelled_at` and `updated_at`, so an approval saved meanwhile keeps its time.
- **Member row lock on insert (2.6):** `CreateBooking` reads the member `FOR SHARE` before inserting and refuses a deleted one with `ErrMemberDeleted`. `DeleteMember`'s `UPDATE` conflicts with that lock, so a booking and a deletion of one member queue on the row. The foreign key's `FOR KEY SHARE` doesn't conflict with it, so it isn't enough, and an `INSERT ... WHERE NOT deleted` without the lock reads a stale row. An unknown member now comes from this read rather than the foreign key.
- **Lock, then sweep (2.6):** `SaveDeletion` marks the member deleted (taking the row lock), saves the bookings the service cancelled, then cancels at `m.DeletedAt()` any other booking of the member that isn't cancelled and hasn't ended, the ones made between the service's list and the save. The interface stays as it was. Cost: "which bookings a deletion cancels" is in SQL as well as in `cancelBookings`. Rejected: sweeping alone (a parameter Postgres would ignore) and a unit-of-work interface (bigger change for one race).
- **`ErrMemberDeleted` from `CreateBooking` in Postgres only (2.6):** the `cowbook.BookingStore` contract doesn't promise it, and memstore keeps its simpler behavior.
- **`RestoreMember` goes through `NewMember` (2.6):** as `RestoreSpace` does; `MemberRecord` embeds `MemberParams`.
- **Ordering tests hold a transaction (2.6):** besides races, two tests hold one side's lock in a test transaction, wait until `pg_blocking_pids` shows the store's call waiting, then commit, so both orders of booking and deleting are tested every run.
- **`repl.Store` in the REPL package (2.7):** the session declares the seven calls its commands make (`LocationFor`, `CreateSpace`, `GetSpace`, the three lists, `CreateMember`), and both stores meet it without naming it. Changes still go through the services.
- **`postgres.Stores` embeds the three stores (2.7):** one value with every method, which meets the `cowbook` interfaces and `repl.Store`. It lives in `postgres`, not `main` as first planned, so the REPL's integration test builds the same thing; M3 can use it per request.
- **IDs from the store in memory too (2.7):** `memstore.CreateSpace` and `CreateMember` replace `AddSpace` and `AddMember`, where the caller picked the ID; mixing both would collide. The memstore tests' spaces are now 1 and 2, not 7 and 8.
- **`ErrEmailTaken` in `cowbook` (2.7):** a store error beside `ErrMemberNotFound`. Postgres maps `23505` on `members_email_key`; memstore checks under its mutex. The REPL calls `NewMember` then the store, as for spaces; a `MemberService.Create` waits for 3.7.
- **`LookupTenant` opens `active` tenants only (2.7):** a `provisioning` one may have no schema, a `suspended` one must stay blocked. `ErrTenantNotFound` lives in `postgres`, since there is no `Tenant` domain type yet.
- **`flag` for `repl` now (2.7):** `flag.NewFlagSet("repl", ContinueOnError)`; bad flags exit 2, `-h` exits 0. The `switch` on `os.Args[1]` and the other subcommands move in 3.2.
- **Stdlib `ServeMux` until 3.3 (3.1):** one route doesn't need chi; Go 1.22 method patterns give 405 and 404 for free. chi arrives in 3.3 as planned, with the tenant routes and middleware.
- **`/healthz` at the root, pinging Postgres (3.1):** platform-level, no tenant. 200 or 503 within a 2 s ping timeout; the reason goes to the log only, never the body. Rejected: a liveness-only check, and separate `/healthz` and `/readyz`.
- **Fail fast at startup (3.1):** `serve` pings once before listening, so a wrong `DATABASE_URL` or a stopped database exits 1 instead of serving 503s.
- **`localhost:8080` by default (3.1):** not `:8080`, which listens on every interface; nothing is fit for a network before login (M4) and database roles (5.1).
- **Config read through an injected `getenv` (3.1):** `loadServeConfig(os.Getenv)` in the binary, a map in the tests. Three variables: `DATABASE_URL`, `COWBOOK_ADDR`, `COWBOOK_LOG_LEVEL`. Timeouts are constants until something needs to change them.
- **The composition root opens the listener (3.1):** `server.Run` takes a `net.Listener`, so tests use port 0 and the log shows the real address.
- **`internal/server` apart from `internal/httpapi` (3.1):** the run loop serves whatever handler it's given, so the web pages (M7) share it without depending on the API package.
- **Server timeouts set (3.1):** header 5 s, request 15 s, response 15 s, idle 60 s, against slow or idle clients holding connections. `ErrorLog` goes through `slog`.
- **Second signal kills (3.1):** `context.AfterFunc(ctx, stop)` hands SIGINT and SIGTERM back to Go's default once shutdown starts. Shutdown waits 10 s, then `serve` exits 1. The pool closes after the server, by `defer` order.
- **No request log yet (3.1):** middleware is 3.3's topic.

## How each README rule is enforced

| README rule | Enforced by |
|---|---|
| No overlapping bookings; only one simultaneous winner | Exclusion constraint in each tenant schema on `(space_id, tstzrange(starts_at, ends_at))` where not cancelled; Postgres error `23P01` maps to `ErrSlotTaken` |
| End after start, within opening hours, capacity, max length | Domain constructor `cowbook.NewBooking`, which also refuses unavailable spaces and starts in the past (Agent's call, open to rejection) |
| Only the booker or staff can cancel | Domain rule in `Booking.Cancel`, which the service calls: the booker, an admin, or front desk assigned to that location |
| Cancelling frees the slot immediately | `cancelled_at` set in one transaction; the constraint ignores cancelled rows; history is kept |
| No operator sees another's data | A schema per tenant, plus a per-tenant DB role that has no access to other schemas; other tenants' resources return 404 |
| Support can manage accounts, not member data or bookings | Separate DB role granted only the `staff` table in each tenant schema, so the database refuses even if the code is wrong |
| Every support action recorded | Audit row written in the same transaction as the action; role can only insert and read |
| One customer must not hurt others | Per-operator rate limit by plan (`golang.org/x/time/rate`), query timeouts |
| Fast onboarding | Self-service signup: tenant row marked `provisioning`, schema and role created, migrations and first admin applied, then marked `active`. On failure the schema and role are dropped |

## Architecture

Layering: delivery (`httpapi`, `web`, `main`) -> `cowbook` services -> `cowbook` domain; `postgres` implements interfaces defined in `cowbook`. The domain imports no database or HTTP packages. The clock is injected.

```text
cmd/cowbook/main.go      the one binary and composition root: repl, serve, migrate, ...
cowbook/                 domain + services
internal/postgres/       store implementations, sqlc output
internal/httpapi/        JSON API at /api/o/{slug}/..., plus /healthz
internal/server/         runs a handler until a signal, then shuts down gracefully
internal/web/            HTML pages at /o/{slug}/..., embedded templates
migrations/platform/     goose SQL, run once: tenants, support users, audit log
migrations/tenant/       goose SQL, run in every tenant schema
compose.yaml, justfile, sqlc.yaml
```

### Domain model

Agent's calls, open to rejection at each step.

- `Tenant`: slug, name, plan, status (provisioning, active or suspended). The tenant comes from the URL slug. Its schema name is `tenant_<id>`, built from the numeric ID and never from the slug or other user input.
- `Location`: name, IANA timezone. Added for chains with many locations.
- `Space`: location, name, kind, capacity, max booking length, weekly opening hours, available flag.
- `Booking`: space, member, attendees (new, needed for the capacity rule), start, end, `cancelledAt`. A booking must fit inside one opening window, so it cannot cross midnight.
- `Member` and `Staff`: **separate tables**, so support can be granted access to staff accounts and denied member data. Email is unique per tenant. Members are soft-deleted with `deletedAt`. `Staff` has a role (admin or front desk); a `staff_locations` table holds front desk assignments.
- `SupportUser`, `AuditEntry`: platform level, no tenant.

Type conventions:

- IDs are `int64`, matching Postgres `bigint` and sqlc.
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
- `just test-integration`: `-tags integration` against Postgres in compose, using `-race`. Includes the concurrent-booking test and the isolation suite.
- CLI exit codes: `0` ok, `1` general error, `2` usage error.

### Dependencies

Added at the step that first needs them, for approval then:

`go-chi/chi/v5`, `jackc/pgx/v5`, `x/crypto`, `x/time`, and as `go tool` entries `sqlc` and `goose`. Image: `postgres:18`.

Already added: `staticcheck`, goose (v3.29.0) and sqlc (v1.31.1), as `go tool` entries; `jackc/pgx/v5`; image `postgres:18`.

## Risks and trade-offs

- **Schema per tenant, benefits:** strong separation that's easy to see, and one operator can be exported, restored or moved to a dedicated database on its own.
- **Schema per tenant, costs at scale:** every operator adds its own tables, indexes and role, so thousands of operators slow backups and catalog lookups and weigh on the README's "economical as operators grow". This won't show up locally. A free tier with many small operators is the case to watch.
- **Migrations across tenants:** a change runs once per schema, so it takes longer, can fail partway, and leaves tenants on different versions for a while. Old code must keep working during a rollout. 5.5 handles reporting and re-runs.
- **Signup runs DDL:** a public request ends up creating schemas and roles, so signup needs its own strict rate limit and a separate, more privileged pool.
- **Reused connections:** a mistake here could expose the wrong tenant. Since 2.4 the `search_path` is transaction-local, and a test on one reused connection proves it's gone after a commit, an error and a panic. pgx's prepared statement cache fails while schemas are on different versions, so the pool uses `describe_exec`, at one extra round trip per query.
- **sqlc's cost:** it parses SQL with Postgres's parser through cgo, so building the tool needs a C compiler and takes about 30 seconds the first time. It adds about 30 indirect entries to `go.mod`. `go build` of the app needs neither, since the generated code is committed.
- **Shared instance:** one operator's heavy use can still affect others beyond what rate limits catch, because schemas don't separate CPU or disk. Dedicated databases remain a documented future option.
- **goose's driver list in `go.mod`:** the goose command imports drivers for many databases, so `go.mod` and `go.sum` carry dozens of indirect entries that `cowbook` never builds. `cowbook migrate` (3.2) uses goose as a library with pgx only; the tool entry could then go.
- **Interrupted provisioning:** cleanup after a failed step is done by hand, because goose runs its own transactions. If the process dies mid-way, the tenant row stays at `provisioning`, maybe with a schema. Signup (5.3) must handle rows stuck there.
- **Integration tests share the dev database:** they only create `test-` tenants and remove them, but they use up tenant ids.
- **Superuser until 5.1:** the compose database's `cowbook` user owns everything and every command connects as it.
- **In-memory rate limiter:** works for one process only; multiple instances would need a shared store. Acceptable since this runs locally.
- **Limited support view:** with no member or booking totals, support can't tell how busy an operator is, and the platform can't estimate cost per operator from this view. That part of the README stays open.
- **Public registration:** anyone can create a member account on an operator's page, with no email check. The per-IP limit slows abuse but doesn't verify identity.
- **Per-tenant roles arrive in M5:** schemas and `WithTenant` exist from M2, so nothing needs reshaping, but the second lock and the isolation proof come later. M3 and M4 run only locally.
- **Opening hours on a clock change:** `WeeklyHours.Contains` checks only the two ends of a booking. If the clock goes back during a booking and a window opens or closes inside the repeated hour, the booking can pass while briefly outside the window. Example: New York, 1 Nov 2026, window 00:00-01:30, booking 01:00 EDT to 01:20 EST. A test pins this behavior. The fix is a loop over `Time.ZoneBounds`, not worth the code for one hour a year.
- **Member soft delete, still open:** `MemberService.Delete` records the deletion and cancels the member's unended bookings.
  - Who may delete: no actor check yet. Member management arrives in 3.7 and role checks in 4.6.
  - Acting as a deleted member: `BookingService.Book`, `Cancel` and `Approve` refuse it. Login must also refuse deleted members (4.2).
  - Deletion reads, then saves: `MemberService.Delete` lists the bookings to cancel before `SaveDeletion`. Since 2.6 the Postgres `SaveDeletion` also cancels any booking made in between, in its own transaction; the in-memory store doesn't.
  - Email reuse: decided in 2.2. The partial unique index `members_email_key` (`WHERE deleted_at IS NULL`) lets a deleted member's email register again.
  - Personal data: soft delete keeps the name and email. Erasing them on request is not covered.
- **Pending bookings can block slots:** staff can hold a slot for up to 24 hours with a booking the member never approves. Visible in the day's view; a shorter deadline or a per-staff limit would be the fix.
- **Deleting and booking at once:** closed in Postgres by 2.6: both lock the member row, so a booking is either refused or cancelled by the deletion. The in-memory store still lets one slip in.
- **Unexported domain fields:** `Booking`, `Space` and `Member` keep their fields unexported. 1.8 added the read-only accessors `internal/memstore` needs; 2.4b added the rest for `Space` and `RestoreSpace`, 2.5 for `Booking` and `RestoreBooking`. 2.6 added the same for `Member` and `RestoreMember`.
- **Timestamp precision:** Go keeps nanoseconds, Postgres microseconds, and pgx drops the rest when it stores a time. A value just created and the same value read back aren't `Equal` unless the clock gave whole microseconds. The booking tests compare after cutting to the microsecond (2.5). Lazy expiry and the approval check compare stored times with the clock's, so a deadline is at most a microsecond early; nothing depends on that.
- **Booking IDs can be probed:** `BookingService.Cancel` and `BookingService.Approve` return `ErrBookingNotFound` before they check permission, so a member can tell which booking IDs exist within their own operator. Revisit at 3.6.
- **The in-memory store is simpler than Postgres:** `internal/memstore` has no lazy expiry (kept that way in 2.5), doesn't refuse a booking for a member deleted at the same moment, and replaces whole bookings on save, so a cancel racing an approval drops the approval time. Fine for tests; the Postgres store doesn't copy them: 2.5 expires lapsed bookings and writes only the approval columns, and 2.6 writes only the cancel columns and handles deletion.
- **Lapsed bookings stay uncancelled until needed:** lazy expiry runs only when a new booking overlaps one. Until then the row looks active, so availability (3.4) and listings (3.6) must use `State(now)`, not `cancelled_at`, to show the slot free.
- **One more statement per booking:** since 2.6 `CreateBooking` reads and share-locks the member row first. Bookings of one member don't wait for each other; they wait only for that member's deletion.
- **Exclusion constraint under contention:** Postgres checks the constraint after adding the index entry, so two conflicting inserts can in principle wait on each other and one end with a deadlock error (`40P01`) instead of `23P01`. Not seen in 120 runs of the 2.5 race tests on a 16-connection pool. If it appears, `CreateBooking` should retry the transaction once.
- **REPL clock after a restart (2.7):** with `-store=postgres` the data stays but the clock starts again at 16 November 08:00. A booking made after `clock add 48h` is in the future again, and the next run's `clock add` may put earlier bookings in the past. `clock set` gets back. Decided by the user over the wall clock.
- **`LocationFor` is REPL-only (2.7):** a find-or-create by timezone in both stores, for `space add`. Two sessions adding a new zone at once can both insert a row; harmless, since the lowest id wins. 3.7 replaces it with location management.
- **`/healthz` is unauthenticated and hits the database (3.1):** one round trip per call, open to anyone who can reach the port. Fine on `localhost`; 5.4's rate limits would cover it if it's ever exposed.
- **No request log until 3.3 (3.1):** while exercising the server with `curl`, only startup, shutdown and failed health checks show in the log.
- **A second SIGINT in scripts (3.1):** a job started with `&` from a shell without job control begins with SIGINT ignored, and `stop` restores that, so in a script only a second SIGTERM kills. In a terminal the second Ctrl-C kills, as intended.
- **Staff booking that starts right away:** its approval deadline is its start, which is now, so it can never be approved. It holds the slot until lazy expiry cancels it. Refusing staff bookings that start within some minimum lead time would fix it.
