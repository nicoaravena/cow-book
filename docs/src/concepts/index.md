# Topic index

Each topic links to the steps where it shows up.

| Topic | Steps |
| --- | --- |
| Accessor methods on unexported fields | [1.8](1.8-in-memory-store.md#values-copies-and-accessors), [1.10](1.10-repl-catches-up.md#more-accessors), [2.4b](2.4b-space-store.md#rebuilding-through-the-constructor) |
| Applied migrations never change | [2.3](2.3-bookings.md#a-new-migration-not-an-edited-one) |
| Arrays vs slices | [1.2](1.2-opening-hours.md#arrays-slices-and-sorting) |
| `bufio.Scanner`, reading lines | [1.9](1.9-repl.md#reading-lines-with-bufioscanner) |
| Build tags (`//go:build`), integration tests | [2.2](2.2-tenant-schemas.md#integration-tests-and-build-tags), [2.4](2.4-sqlc-and-withtenant.md#build-tags-again) |
| Check constraints, named constraints | [2.1](2.1-postgres-and-migrations.md#the-databases-own-rules) |
| Closures, for transaction scope | [2.4](2.4-sqlc-and-withtenant.md#closures-for-transaction-scope) |
| Closing a channel to release many goroutines | [1.8](1.8-in-memory-store.md#goroutines) |
| Compile-time interface checks (`var _ I = (*T)(nil)`) | [1.8](1.8-in-memory-store.md#compile-time-interface-checks), [2.4b](2.4b-space-store.md#a-store-bound-to-one-tenant), [2.7](2.7-repl-on-postgres.md#struct-embedding-three-stores-as-one-value) |
| Compose, volumes, healthchecks | [2.1](2.1-postgres-and-migrations.md#compose) |
| Connection pools (`pgxpool`), reused connections | [2.4](2.4-sqlc-and-withtenant.md#a-pool-and-why-connections-get-reused) |
| Composition root (`main` wires dependencies) | [1.9](1.9-repl.md#a-delivery-adapter), [2.7](2.7-repl-on-postgres.md#interfaces-made-of-interfaces), [3.1](3.1-serve.md#shutting-down-in-order) |
| Config from the environment, injected `getenv` | [3.1](3.1-serve.md#config-from-the-environment) |
| Constructors (`NewX`) and params structs | [1.1](1.1-space.md#constructors) |
| `context.AfterFunc`, a second Ctrl-C | [3.1](3.1-serve.md#signals-signalnotifycontext) |
| `context.Context` | [1.5](1.5-services-and-stores.md#contextcontext) |
| `context.WithoutCancel` | [2.2](2.2-tenant-schemas.md#cleaning-up-after-a-failure), [2.4](2.4-sqlc-and-withtenant.md#defer-and-rolling-back), [3.1](3.1-serve.md#graceful-shutdown) |
| `context.WithTimeout`, deadlines on a request's context | [3.1](3.1-serve.md#the-health-check) |
| Data races and `go test -race` | [1.8](1.8-in-memory-store.md#the-race-detector) |
| Deadlocks and lock order | [2.6](2.6-cancel-and-delete.md#lock-order-and-deadlocks) |
| `database/sql` vs pgx, `stdlib.OpenDB` | [2.2](2.2-tenant-schemas.md#databasesql-and-pgx) |
| Daylight saving time | [1.2](1.2-opening-hours.md#daylight-saving-time) |
| Defensive copies (`slices.Clone`) | [1.4](1.4-members-and-actors.md#defensive-copies) |
| `defer` with rollback, panics and `recover` | [2.4](2.4-sqlc-and-withtenant.md#defer-and-rolling-back) |
| Dependency injection, injected clock | [1.5](1.5-services-and-stores.md#dependency-injection-including-the-clock), [2.2](2.2-tenant-schemas.md#faking-a-file-system-with-fstestmapfs) |
| Durations (`time.Duration`) | [1.2](1.2-opening-hours.md#durations) |
| Email validation (`net/mail`) | [1.4](1.4-members-and-actors.md#validating-emails) |
| `embed` and `fs.Sub` | [2.2](2.2-tenant-schemas.md#embedding-files-with-embed) |
| Embedded structs, promoted fields and methods | [2.4b](2.4b-space-store.md#embedded-structs), [2.5](2.5-booking-store.md#rebuilding-without-the-constructor), [2.7](2.7-repl-on-postgres.md#struct-embedding-three-stores-as-one-value) |
| Embedding interfaces in interfaces | [2.7](2.7-repl-on-postgres.md#interfaces-made-of-interfaces) |
| `.env` and `.env.example` | [2.1](2.1-postgres-and-migrations.md#env-and-envexample) |
| `errors.As`, `pgconn.PgError` | [2.2](2.2-tenant-schemas.md#checking-which-constraint-refused-a-row), [2.3](2.3-bookings.md#exclusion-constraints), [2.4b](2.4b-space-store.md#foreign-key-errors), [2.5](2.5-booking-store.md#one-error-several-meanings-errorsas-and-a-switch), [2.7](2.7-repl-on-postgres.md#creating-members-and-a-taken-email) (unique index) |
| `errors.Is` | [1.3](1.3-booking-rules.md#errorsis), [2.2](2.2-tenant-schemas.md#cleaning-up-after-a-failure) |
| `errors.Join` | [2.2](2.2-tenant-schemas.md#cleaning-up-after-a-failure) |
| Exclusion constraints, GiST, `btree_gist` | [2.3](2.3-bookings.md#exclusion-constraints) |
| Exit codes for a CLI | [2.7](2.7-repl-on-postgres.md#flags-for-one-subcommand-flagnewflagset), [3.1](3.1-serve.md#exit-codes) |
| Exported vs unexported names | [1.1](1.1-space.md#exported-and-unexported-names) |
| `flag`, `flag.NewFlagSet` per subcommand | [2.7](2.7-repl-on-postgres.md#flags-for-one-subcommand-flagnewflagset), [3.1](3.1-serve.md#exit-codes) |
| `fstest.MapFS` | [2.2](2.2-tenant-schemas.md#faking-a-file-system-with-fstestmapfs) |
| Functions as values | [1.5](1.5-services-and-stores.md#dependency-injection-including-the-clock), [3.1](3.1-serve.md#config-from-the-environment) |
| Generated code (sqlc, `DO NOT EDIT`) | [2.4](2.4-sqlc-and-withtenant.md#generated-code-with-sqlc) |
| `go tool` dependencies | [0.1](0.1-foundation.md#go-tool-dependencies), [2.1](2.1-postgres-and-migrations.md#go-tool-again) |
| `go vet`, `gofmt`, `staticcheck` | [0.1](0.1-foundation.md#the-justfile) |
| Goroutines | [1.8](1.8-in-memory-store.md#goroutines) |
| Graceful shutdown, `Shutdown` vs `Close` | [3.1](3.1-serve.md#graceful-shutdown) |
| Guarded `UPDATE`, `:execrows`, `RowsAffected` | [2.5](2.5-booking-store.md#checking-and-writing-in-one-statement), [2.6](2.6-cancel-and-delete.md#cancelling-one-more-guarded-update) |
| Health checks, liveness vs readiness | [3.1](3.1-serve.md#the-health-check) |
| `http.Handler`, `http.HandlerFunc`, handlers returned by functions | [3.1](3.1-serve.md#the-health-check) |
| `http.Server` timeouts, slowloris | [3.1](3.1-serve.md#httpserver-and-its-timeouts) |
| `http.ServeMux` method patterns (Go 1.22), 404 and 405 | [3.1](3.1-serve.md#routes-with-methods-in-httpservemux) |
| `httptest`: `NewRequest`, `NewRecorder` | [3.1](3.1-serve.md#testing-a-server-httptest-and-a-real-listener) |
| Identity columns, sequence gaps | [2.1](2.1-postgres-and-migrations.md#identity-columns) |
| Implicit interfaces | [1.5](1.5-services-and-stores.md#implicit-interfaces) |
| Interfaces defined by the consumer | [1.5](1.5-services-and-stores.md#implicit-interfaces), [2.7](2.7-repl-on-postgres.md#interfaces-defined-by-the-consumer), [3.1](3.1-serve.md#the-health-check) |
| Instants vs wall-clock time, `Time.In` | [1.2](1.2-opening-hours.md#instants-vs-wall-clock) |
| Integer conversions (`int32(x)`), wrap-around | [2.4b](2.4b-space-store.md#mapping-rows-to-domain-types) |
| Isolation levels, READ COMMITTED re-checks, row locks | [2.5](2.5-booking-store.md#many-at-once), [2.6](2.6-cancel-and-delete.md#what-the-waiting-side-sees) |
| `internal/` packages | [1.8](1.8-in-memory-store.md#internal) |
| `io.Reader` and `io.Writer` | [1.9](1.9-repl.md#ioreader-and-iowriter) |
| `iota` enums with a safe zero value | [1.4](1.4-members-and-actors.md#small-enums-with-iota) |
| Lazy expiry, conditions a constraint can't hold | [2.5](2.5-booking-store.md#lazy-expiry) |
| Lining up columns (`text/tabwriter`, `Flush`) | [1.10](1.10-repl-catches-up.md#lining-up-columns-with-texttabwriter) |
| Listening on port 0, `net.Listener` | [3.1](3.1-serve.md#listening-on-port-0) |
| `log/slog`, structured logs, levels | [3.1](3.1-serve.md#structured-logs-with-logslog) |
| Logic races vs data races | [1.8](1.8-in-memory-store.md#the-race-detector) |
| Loop variables per iteration (Go 1.22) | [1.8](1.8-in-memory-store.md#goroutines) |
| Map iteration order | [1.9](1.9-repl.md#a-map-of-command-functions), [1.10](1.10-repl-catches-up.md#map-order-and-slicessortfunc) |
| Mapping rows to domain types, `pgtype.Interval` | [2.4b](2.4b-space-store.md#mapping-rows-to-domain-types) |
| Maps of functions, method values | [1.9](1.9-repl.md#a-map-of-command-functions) |
| Methods and receivers | [1.2](1.2-opening-hours.md#methods), [1.4](1.4-members-and-actors.md#pointer-vs-value-receivers) |
| Migrations with goose, up and down | [2.1](2.1-postgres-and-migrations.md#migrations-with-goose) |
| Modules, packages, `go.mod` | [0.1](0.1-foundation.md#modules-and-packages) |
| N+1 queries, grouping rows with a map | [2.4b](2.4b-space-store.md#two-queries-not-one-per-space) |
| `NULL` vs Go zero values | [2.2](2.2-tenant-schemas.md#the-tenant-tables), [2.3](2.3-bookings.md#null-and-gos-zero-time), [2.4b](2.4b-space-store.md#mapping-rows-to-domain-types), [2.5](2.5-booking-store.md#rebuilding-without-the-constructor) |
| Partial unique indexes and constraints | [2.2](2.2-tenant-schemas.md#the-tenant-tables), [2.3](2.3-bookings.md#a-partial-constraint-and-why-now-cant-be-in-it), [2.7](2.7-repl-on-postgres.md#creating-members-and-a-taken-email) |
| `pgx.Identifier`, SQL names vs parameters | [2.2](2.2-tenant-schemas.md#building-sql-names-safely-pgxidentifier) |
| pgx query exec modes, stale prepared plans | [2.4](2.4-sqlc-and-withtenant.md#query-exec-modes-and-stale-plans) |
| Pointer vs value receivers | [1.4](1.4-members-and-actors.md#pointer-vs-value-receivers) |
| Postgres schemas and `search_path` | [2.1](2.1-postgres-and-migrations.md#three-kinds-of-schema) |
| psql `ON_ERROR_ROLLBACK` | [2.1](2.1-postgres-and-migrations.md#on_error_rollback) |
| Range copies; changing slice elements by index | [1.7](1.7-deleting-a-member.md#slices-of-values-changing-elements-in-a-loop) |
| Range types, `tstzrange` and `[)` bounds | [2.3](2.3-bookings.md#ranges-and-half-open-bounds) |
| Read, check, write: the store decides the winner | [1.5](1.5-services-and-stores.md#read-check-write), [1.8](1.8-in-memory-store.md#why-the-store-decides-the-winner), [2.5](2.5-booking-store.md#checking-and-writing-in-one-statement), [2.6](2.6-cancel-and-delete.md#lock-then-sweep) (reads that go stale) |
| Rebuilding domain types through the constructor | [2.4b](2.4b-space-store.md#rebuilding-through-the-constructor), [2.5](2.5-booking-store.md#rebuilding-without-the-constructor) (when it can't), [2.6](2.6-cancel-and-delete.md#rebuilding-a-member) |
| Row lock modes (`FOR SHARE`, `FOR KEY SHARE`, ...), foreign key locks | [2.6](2.6-cancel-and-delete.md#row-locks) |
| Rules the database checks vs rules in Go | [2.3](2.3-bookings.md#rules-the-database-checks-and-rules-it-leaves-to-go) |
| Saving a batch all or nothing | [1.7](1.7-deleting-a-member.md#a-batch-saved-all-or-nothing), [2.6](2.6-cancel-and-delete.md#transactions-spanning-several-queries) |
| `search_path` and `current_schema()` | [2.2](2.2-tenant-schemas.md#one-goose-table-per-schema) |
| Sentinel errors | [1.1](1.1-space.md#sentinel-errors), [1.9](1.9-repl.md#errors-for-control-flow) |
| Signals, `signal.NotifyContext`, inherited dispositions | [3.1](3.1-serve.md#signals-signalnotifycontext) |
| Soft delete | [1.4](1.4-members-and-actors.md#soft-delete), [1.7](1.7-deleting-a-member.md) |
| Sorting with `slices.SortFunc` and `cmp.Compare` | [1.10](1.10-repl-catches-up.md#map-order-and-slicessortfunc) |
| Splitting strings (`Fields`, `Cut`, `SplitSeq`) | [1.9](1.9-repl.md#splitting-words) |
| State from zero-value timestamps | [1.3](1.3-booking-rules.md#cancelled-as-a-timestamp), [1.6](1.6-who-can-book.md#state-from-zero-value-timestamps), [1.10](1.10-repl-catches-up.md#a-bookings-state-belongs-to-the-domain) |
| String enums (`type SpaceKind string`) | [1.1](1.1-space.md#string-enums), [1.10](1.10-repl-catches-up.md#a-bookings-state-belongs-to-the-domain) |
| `switch` with no value, first broken rule wins | [1.1](1.1-space.md#constructors), [1.3](1.3-booking-rules.md#the-rules-as-one-switch) |
| `sync.Mutex` and `defer Unlock` | [1.8](1.8-in-memory-store.md#syncmutex) |
| `sync.WaitGroup` and `wg.Go` | [1.8](1.8-in-memory-store.md#goroutines) |
| Table-driven tests and subtests (`t.Run`) | [1.1](1.1-space.md#table-driven-tests), [2.5](2.5-booking-store.md#subtests-and-shared-test-helpers) |
| Terminal detection (`os.ModeCharDevice`) | [1.9](1.9-repl.md#terminal-or-script) |
| Test fakes | [1.5](1.5-services-and-stores.md#implicit-interfaces), [3.1](3.1-serve.md#the-health-check) |
| Testing an order of events (`pg_blocking_pids`, held transactions) | [2.6](2.6-cancel-and-delete.md#testing-an-order-of-events) |
| Test helpers (`t.Helper`) | [1.2](1.2-opening-hours.md#test-helpers), [2.5](2.5-booking-store.md#subtests-and-shared-test-helpers) |
| Tests, `t.Errorf` vs `t.Fatalf` | [0.1](0.1-foundation.md#tests) |
| Tests that can fail: removing the guard | [2.6](2.6-cancel-and-delete.md#proving-the-tests-can-fail), [3.1](3.1-serve.md#testing-a-server-httptest-and-a-real-listener) |
| Time layouts, `time.Parse`, `time.Date` normalizing | [1.9](1.9-repl.md#parsing-times) |
| Timestamp precision, Go nanoseconds vs Postgres microseconds | [2.4b](2.4b-space-store.md#timestamps-lose-their-nanoseconds), [2.5](2.5-booking-store.md#comparing-bookings) |
| Transaction-local settings (`set_config`, `SET LOCAL`) | [2.4](2.4-sqlc-and-withtenant.md#settings-that-end-with-the-transaction) |
| Transactions spanning several queries | [2.6](2.6-cancel-and-delete.md#transactions-spanning-several-queries) |
| Unexported helper methods | [1.6](1.6-who-can-book.md#unexported-helper-methods), [1.7](1.7-deleting-a-member.md#a-rule-without-an-actor) |
| Validate everything before changing anything | [1.4](1.4-members-and-actors.md#validate-everything-before-changing-anything) |
| Value types | [1.2](1.2-opening-hours.md#value-types) |
| Wrapping errors with `%w` | [1.3](1.3-booking-rules.md#wrapping-errors-with-w) |
| Zero values | [0.1](0.1-foundation.md#zero-values) |
