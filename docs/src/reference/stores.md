# Stores

The services in `cowbook` reach storage only through three small interfaces, defined in
`cowbook/service.go`. Every method takes the caller's `context.Context`. Errors are sentinels,
compared with `errors.Is`.

Domain rules run on a copy the service read earlier, so two requests can both pass them. Where
that matters, the store's save decides the winner and gives the loser an error. Those errors are
listed under **Loser gets**.

## `BookingStore`

| Method | Does | Not found | Loser gets |
| --- | --- | --- | --- |
| `CreateBooking(ctx, b)` | Stores `b`, returns it with its ID set | - | `ErrSlotTaken` if `b` overlaps a booking of the same space that isn't cancelled. Pending bookings hold their slot |
| `GetBooking(ctx, id)` | Returns the booking | `ErrBookingNotFound` | - |
| `SaveCancellation(ctx, b)` | Stores a booking `Cancel` changed | `ErrBookingNotFound` | `ErrAlreadyCancelled` |
| `SaveApproval(ctx, b)` | Stores a booking `Approve` changed | `ErrBookingNotFound` | `ErrAlreadyCancelled` or `ErrNotPending`; from Postgres also `ErrApprovalExpired` |
| `ListMemberBookingsEndingAfter(ctx, memberID, t)` | The member's bookings that aren't cancelled and end after `t` | - | - |

Bookings don't include their end: one ending at 10:00 and one starting at 10:00 don't overlap.

## `SpaceStore`

| Method | Does | Not found |
| --- | --- | --- |
| `GetSpace(ctx, id)` | Returns the space and its location's timezone, never nil | `ErrSpaceNotFound` |

## `MemberStore`

| Method | Does | Not found | Loser gets |
| --- | --- | --- | --- |
| `GetMember(ctx, id)` | Returns the member, deleted or not | `ErrMemberNotFound` | - |
| `SaveDeletion(ctx, m, cancelled)` | Stores a member `Delete` changed and the bookings cancelled because of it, all or nothing. A booking someone else cancelled meanwhile keeps that cancellation, with no error | `ErrMemberNotFound` | `ErrMemberDeleted` |

## Implementations

### `internal/memstore`

`memstore.Store` implements all three, in maps, for tests and local runs. It is safe for many
goroutines: every method holds one `sync.Mutex` for its whole run, so each check and its write
happen as one step.

- `memstore.New()` returns an empty store.
- `LocationFor(ctx, tz)` returns the location in `tz`, adding one if there is none; zones compare
  by name (2.7).
- `CreateSpace(ctx, sp)` and `CreateMember(ctx, m)` store with the next ID and return it set
  (2.7). `CreateSpace` takes the timezone from `sp`'s location and returns `ErrInvalidLocation`
  for an unknown one; `CreateMember` returns `ErrEmailTaken` if a member who isn't deleted has the
  email. They replace 1.8's `AddSpace` and `AddMember`, where the caller picked the ID.
- `ListSpaces(ctx)`, `ListMembers(ctx)`, `ListBookings(ctx)` return everything stored, sorted by
  ID, cancelled bookings and deleted members included. They are for the REPL and are not part of
  the interfaces above: no service needs them yet (1.10).
- Every method returns `ctx.Err()` without doing anything if the context is already done.

Differences from the Postgres store:

- No lazy expiry: a pending booking past its approval deadline keeps holding its slot. Kept that
  way on purpose (2.5).
- `CreateBooking` doesn't refuse a member deleted at the same moment. Postgres does (2.6).
- Saves replace the whole booking, so a cancel that read the booking before an approval was saved
  drops the approval time. The booking still ends cancelled.

### `internal/postgres`

`postgres.SpaceStore` (2.4b), in `internal/postgres/spaces.go`, implements `SpaceStore` for one
tenant: `NewSpaceStore(pool, tenantID)`. It holds no connection, and every method runs in one
`WithTenant` transaction, so it sees only that tenant's schema.

| Method | Does | Errors |
| --- | --- | --- |
| `CreateSpace(ctx, sp)` | Stores `sp` and one `opening_windows` row per window, all or nothing; returns it with its ID set. Timestamps are `sp`'s, not `now()` | `ErrInvalidLocation` if the tenant has no such location (`23503` on `spaces_location_id_fkey`) |
| `GetSpace(ctx, id)` | The space and its location's timezone | `ErrSpaceNotFound`; a wrapped error if the stored timezone isn't a real zone |
| `ListSpaces(ctx)` | Every space, sorted by ID, in two queries | - |
| `LocationFor(ctx, tz)` | The id of the lowest location with timezone `tz.String()`, adding one named after the zone if there is none, in one transaction (2.7). For the REPL until 3.7 | - |

Rows are rebuilt with `RestoreSpace`, so a row that breaks a domain rule, such as a space left
with no windows, is an error wrapping that rule's sentinel. Times come back cut to the
microsecond (pgx drops the nanoseconds) and in `time.Local`; compare them with `Equal`.

`postgres.BookingStore` (2.5), in `internal/postgres/bookings.go`, does the same for bookings:
`NewBookingStore(pool, tenantID)`. It implements all of `BookingStore` since 2.6, checked at
compile time.

| Method | Does | Errors |
| --- | --- | --- |
| `CreateBooking(ctx, b)` | In one transaction: locks the member's row `FOR SHARE` until commit (2.6), cancels the space's pending bookings that overlap `b` and whose deadline has passed by `b.CreatedAt()` (lazy expiry), then inserts `b`. If the insert fails, the expiry rolls back with it | `ErrMemberNotFound`, `ErrMemberDeleted` from the member lock; `ErrSlotTaken` (`23P01` on `bookings_no_overlap`); `ErrSpaceNotFound` (`23503` on `bookings_space_id_fkey`) |
| `GetBooking(ctx, id)` | The booking, rebuilt with `RestoreBooking`. A lapsed booking comes back as stored, not cancelled | `ErrBookingNotFound` |
| `SaveCancellation(ctx, b)` | Writes `cancelled_at` and `updated_at` only, in one `UPDATE` that matches only a booking not cancelled yet, so an approval saved meanwhile keeps its time (2.6) | `ErrBookingNotFound`; the loser gets `ErrAlreadyCancelled` |
| `SaveApproval(ctx, b)` | Writes `approved_at` and `updated_at` only, in one `UPDATE` that matches only a booking still pending with its deadline after `b.ApprovedAt()` | `ErrBookingNotFound`; the loser gets `ErrAlreadyCancelled`, `ErrNotPending` or `ErrApprovalExpired` |
| `ListMemberBookingsEndingAfter(ctx, memberID, t)` | The member's bookings not cancelled and ending after `t`, ordered by start. Lapsed pending ones included (2.6) | - |
| `ListBookings(ctx)` | Every booking, sorted by ID, cancelled ones included. A lapsed booking comes back as stored; `State` says expired (2.7) | - |

The store's clock is the booking's: lazy expiry uses `b.CreatedAt()`, which `NewBooking` took
from the service's injected clock, so `CreateBooking` needs no `now` of its own.

`postgres.MemberStore` (2.6), in `internal/postgres/members.go`: `NewMemberStore(pool,
tenantID)`, with `GetMember` and `SaveDeletion`, checked at compile time, and `CreateMember` and
`ListMembers` from 2.7.

| Method | Does | Errors |
| --- | --- | --- |
| `GetMember(ctx, id)` | The member, deleted or not, rebuilt with `RestoreMember` | `ErrMemberNotFound` |
| `CreateMember(ctx, m)` | Stores `m` with its timestamps, returns it with its ID set (2.7) | `ErrEmailTaken` (`23505` on `members_email_key`) |
| `ListMembers(ctx)` | Every member, sorted by ID, deleted ones included (2.7) | - |
| `SaveDeletion(ctx, m, cancelled)` | In one transaction: marks the member deleted only if they aren't yet, which locks the row until commit; saves each booking in `cancelled` that isn't cancelled already; then cancels at `m.DeletedAt()` any other booking of the member not cancelled and not ended, one made after the service's list | `ErrMemberNotFound`; the loser gets `ErrMemberDeleted` |

The member row's lock is what makes deleting and booking at once safe. Whichever transaction
takes it second waits for the first to commit: a booking then sees the member deleted and
refuses, and a deletion then sees the new booking and cancels it.

`postgres.Stores` (2.7), in `internal/postgres/stores.go`, embeds the three:
`NewStores(pool, tenantID)`. Their methods are promoted, so one `Stores` value meets all three
`cowbook` interfaces and `repl.Store`. `cmd/cowbook` builds the REPL's services on it.

`postgres.LookupTenant(ctx, pool, slug)` (2.7), in `internal/postgres/tenant.go`, returns the id
of the tenant with `slug`, `postgres.ErrTenantNotFound` (wrapped with the slug) if there is none,
or an error naming the status if the tenant isn't `active`.

### Test fake

`fakeStore` in `cowbook/service_test.go` implements all three for the service unit tests. It
records each call and can be told to fail, to check that the services pass store errors on. It
decides no races.

## `repl.Store`

Declared in `internal/repl/repl.go` by its one user, the REPL session (2.7). It holds only the
calls the commands make themselves; changes to bookings and deletions go through the services.
`*memstore.Store` and `postgres.Stores` both meet it without naming it.

| Method | Used by |
| --- | --- |
| `LocationFor(ctx, tz)` | `space add` |
| `CreateSpace(ctx, sp)` | `space add` |
| `GetSpace(ctx, id)` | `book`, `space list`, `booking list`, for the space's timezone |
| `ListSpaces(ctx)` | `space list` |
| `CreateMember(ctx, m)` | `member add` |
| `ListMembers(ctx)` | `member list` |
| `ListBookings(ctx)` | `booking list` |

## Reading a booking or member from outside `cowbook`

Fields are unexported, so stores read them through accessors: `Booking.SpaceID`, `MemberID`,
`Start`, `End`, `Cancelled`, `Approved`, and `Member.Deleted`. 1.10 adds `Booking.Attendees`,
`Booking.State`, `Member.Name`, `Email` and `Space.Name`, `Kind`, `Capacity`, `MaxBookingLength`. 2.4b adds `Space.LocationID`,
`Hours`, `Available`, `CreatedAt`, `UpdatedAt`, and `RestoreSpace` to rebuild a `Space` from a row.
2.5 adds `Booking.BookedBy`, `ApprovedAt`, `CancelledAt`, `CreatedAt`, `UpdatedAt` and
`RestoreBooking`. 2.6 adds `Member.CreatedAt`, `UpdatedAt`, `DeletedAt` and `RestoreMember`.
