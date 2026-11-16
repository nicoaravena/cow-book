# Topic index

Each topic links to the steps where it shows up.

| Topic | Steps |
| --- | --- |
| Accessor methods on unexported fields | [1.8](1.8-in-memory-store.md#values-copies-and-accessors) |
| Arrays vs slices | [1.2](1.2-opening-hours.md#arrays-slices-and-sorting) |
| `bufio.Scanner`, reading lines | [1.9](1.9-repl.md#reading-lines-with-bufioscanner) |
| Closing a channel to release many goroutines | [1.8](1.8-in-memory-store.md#goroutines) |
| Compile-time interface checks (`var _ I = (*T)(nil)`) | [1.8](1.8-in-memory-store.md#compile-time-interface-checks) |
| Composition root (`main` wires dependencies) | [1.9](1.9-repl.md#a-delivery-adapter) |
| Constructors (`NewX`) and params structs | [1.1](1.1-space.md#constructors) |
| `context.Context` | [1.5](1.5-services-and-stores.md#contextcontext) |
| Data races and `go test -race` | [1.8](1.8-in-memory-store.md#the-race-detector) |
| Daylight saving time | [1.2](1.2-opening-hours.md#daylight-saving-time) |
| Defensive copies (`slices.Clone`) | [1.4](1.4-members-and-actors.md#defensive-copies) |
| Dependency injection, injected clock | [1.5](1.5-services-and-stores.md#dependency-injection-including-the-clock) |
| Durations (`time.Duration`) | [1.2](1.2-opening-hours.md#durations) |
| Email validation (`net/mail`) | [1.4](1.4-members-and-actors.md#validating-emails) |
| `errors.Is` | [1.3](1.3-booking-rules.md#errorsis) |
| Exported vs unexported names | [1.1](1.1-space.md#exported-and-unexported-names) |
| Functions as values | [1.5](1.5-services-and-stores.md#dependency-injection-including-the-clock) |
| `go tool` dependencies | [0.1](0.1-foundation.md#go-tool-dependencies) |
| `go vet`, `gofmt`, `staticcheck` | [0.1](0.1-foundation.md#the-makefile) |
| Goroutines | [1.8](1.8-in-memory-store.md#goroutines) |
| Implicit interfaces | [1.5](1.5-services-and-stores.md#implicit-interfaces) |
| Instants vs wall-clock time, `Time.In` | [1.2](1.2-opening-hours.md#instants-vs-wall-clock) |
| `internal/` packages | [1.8](1.8-in-memory-store.md#internal) |
| `io.Reader` and `io.Writer` | [1.9](1.9-repl.md#ioreader-and-iowriter) |
| `iota` enums with a safe zero value | [1.4](1.4-members-and-actors.md#small-enums-with-iota) |
| Logic races vs data races | [1.8](1.8-in-memory-store.md#the-race-detector) |
| Loop variables per iteration (Go 1.22) | [1.8](1.8-in-memory-store.md#goroutines) |
| Maps of functions, method values | [1.9](1.9-repl.md#a-map-of-command-functions) |
| Methods and receivers | [1.2](1.2-opening-hours.md#methods), [1.4](1.4-members-and-actors.md#pointer-vs-value-receivers) |
| Modules, packages, `go.mod` | [0.1](0.1-foundation.md#modules-and-packages) |
| Pointer vs value receivers | [1.4](1.4-members-and-actors.md#pointer-vs-value-receivers) |
| Range copies; changing slice elements by index | [1.7](1.7-deleting-a-member.md#slices-of-values-changing-elements-in-a-loop) |
| Read, check, write: the store decides the winner | [1.5](1.5-services-and-stores.md#read-check-write), [1.8](1.8-in-memory-store.md#why-the-store-decides-the-winner) |
| Saving a batch all or nothing | [1.7](1.7-deleting-a-member.md#a-batch-saved-all-or-nothing) |
| Sentinel errors | [1.1](1.1-space.md#sentinel-errors), [1.9](1.9-repl.md#errors-for-control-flow) |
| Soft delete | [1.4](1.4-members-and-actors.md#soft-delete), [1.7](1.7-deleting-a-member.md) |
| Splitting strings (`Fields`, `Cut`, `SplitSeq`) | [1.9](1.9-repl.md#splitting-words) |
| State from zero-value timestamps | [1.3](1.3-booking-rules.md#cancelled-as-a-timestamp), [1.6](1.6-who-can-book.md#state-from-zero-value-timestamps) |
| String enums (`type SpaceKind string`) | [1.1](1.1-space.md#string-enums) |
| `switch` with no value, first broken rule wins | [1.1](1.1-space.md#constructors), [1.3](1.3-booking-rules.md#the-rules-as-one-switch) |
| `sync.Mutex` and `defer Unlock` | [1.8](1.8-in-memory-store.md#syncmutex) |
| `sync.WaitGroup` and `wg.Go` | [1.8](1.8-in-memory-store.md#goroutines) |
| Table-driven tests and subtests (`t.Run`) | [1.1](1.1-space.md#table-driven-tests) |
| Terminal detection (`os.ModeCharDevice`) | [1.9](1.9-repl.md#terminal-or-script) |
| Test fakes | [1.5](1.5-services-and-stores.md#implicit-interfaces) |
| Test helpers (`t.Helper`) | [1.2](1.2-opening-hours.md#test-helpers) |
| Tests, `t.Errorf` vs `t.Fatalf` | [0.1](0.1-foundation.md#tests) |
| Time layouts, `time.Parse`, `time.Date` normalizing | [1.9](1.9-repl.md#parsing-times) |
| Unexported helper methods | [1.6](1.6-who-can-book.md#unexported-helper-methods), [1.7](1.7-deleting-a-member.md#a-rule-without-an-actor) |
| Validate everything before changing anything | [1.4](1.4-members-and-actors.md#validate-everything-before-changing-anything) |
| Value types | [1.2](1.2-opening-hours.md#value-types) |
| Wrapping errors with `%w` | [1.3](1.3-booking-rules.md#wrapping-errors-with-w) |
| Zero values | [0.1](0.1-foundation.md#zero-values) |
