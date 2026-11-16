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
| `SaveApproval(ctx, b)` | Stores a booking `Approve` changed | `ErrBookingNotFound` | `ErrAlreadyCancelled` or `ErrNotPending` |
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

Differences from the Postgres store planned for M2:

- No lazy expiry: a pending booking past its approval deadline keeps holding its slot.
- `CreateBooking` doesn't refuse a member deleted at the same moment.
- Saves replace the whole booking, so a cancel that read the booking before an approval was saved
  drops the approval time. The booking still ends cancelled.

### Test fake

`fakeStore` in `cowbook/service_test.go` implements all three for the service unit tests. It
records each call and can be told to fail, to check that the services pass store errors on. It
decides no races.

## Reading a booking or member from outside `cowbook`

Fields are unexported, so stores read them through accessors: `Booking.SpaceID`, `MemberID`,
`Start`, `End`, `Cancelled`, `Approved`, and `Member.Deleted`. 1.10 adds `Booking.Attendees`,
`Booking.State`, `Member.Name`, `Email` and `Space.Name`, `Kind`, `Capacity`, `MaxBookingLength`. Rebuilding a `Booking` from a
database row still needs a constructor, due in 2.4.
