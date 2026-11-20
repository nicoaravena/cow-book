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
- `AddSpace(space, loc)` and `AddMember(m)` seed it. The caller sets the ID, since spaces and
  members have no create service yet.
- `ListSpaces(ctx)`, `ListMembers(ctx)`, `ListBookings(ctx)` return everything stored, sorted by
  ID, cancelled bookings and deleted members included. They are for the REPL and are not part of
  the interfaces above: no service needs them yet (1.10).
- Every method returns `ctx.Err()` without doing anything if the context is already done.

Differences from the Postgres store:

- No lazy expiry: a pending booking past its approval deadline keeps holding its slot. Kept that
  way on purpose (2.5).
- `CreateBooking` doesn't refuse a member deleted at the same moment.
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

Rows are rebuilt with `RestoreSpace`, so a row that breaks a domain rule, such as a space left
with no windows, is an error wrapping that rule's sentinel. Times come back cut to the
microsecond (pgx drops the nanoseconds) and in `time.Local`; compare them with `Equal`.

`postgres.BookingStore` (2.5), in `internal/postgres/bookings.go`, does the same for bookings:
`NewBookingStore(pool, tenantID)`. It has three of `BookingStore`'s five methods so far;
`SaveCancellation` and `ListMemberBookingsEndingAfter` arrive in 2.6, and with them the
compile-time interface check.

| Method | Does | Errors |
| --- | --- | --- |
| `CreateBooking(ctx, b)` | In one transaction: cancels the space's pending bookings that overlap `b` and whose deadline has passed by `b.CreatedAt()` (lazy expiry), then inserts `b`. If the insert fails, the expiry rolls back with it | `ErrSlotTaken` (`23P01` on `bookings_no_overlap`); `ErrSpaceNotFound`, `ErrMemberNotFound` (`23503` on `bookings_space_id_fkey`, `bookings_member_id_fkey`) |
| `GetBooking(ctx, id)` | The booking, rebuilt with `RestoreBooking`. A lapsed booking comes back as stored, not cancelled | `ErrBookingNotFound` |
| `SaveApproval(ctx, b)` | Writes `approved_at` and `updated_at` only, in one `UPDATE` that matches only a booking still pending with its deadline after `b.ApprovedAt()` | `ErrBookingNotFound`; the loser gets `ErrAlreadyCancelled`, `ErrNotPending` or `ErrApprovalExpired` |

The store's clock is the booking's: lazy expiry uses `b.CreatedAt()`, which `NewBooking` took
from the service's injected clock, so `CreateBooking` needs no `now` of its own. Members follow
in 2.6 and 2.7.

### Test fake

`fakeStore` in `cowbook/service_test.go` implements all three for the service unit tests. It
records each call and can be told to fail, to check that the services pass store errors on. It
decides no races.

## Reading a booking or member from outside `cowbook`

Fields are unexported, so stores read them through accessors: `Booking.SpaceID`, `MemberID`,
`Start`, `End`, `Cancelled`, `Approved`, and `Member.Deleted`. 1.10 adds `Booking.Attendees`,
`Booking.State`, `Member.Name`, `Email` and `Space.Name`, `Kind`, `Capacity`, `MaxBookingLength`. 2.4b adds `Space.LocationID`,
`Hours`, `Available`, `CreatedAt`, `UpdatedAt`, and `RestoreSpace` to rebuild a `Space` from a row.
2.5 adds `Booking.BookedBy`, `ApprovedAt`, `CancelledAt`, `CreatedAt`, `UpdatedAt` and
`RestoreBooking`. Rebuilding a `Member` arrives in 2.6.
