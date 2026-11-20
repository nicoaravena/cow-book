# Services

`cowbook/service.go`, `cowbook/memberservice.go`. A service loads what the domain rules need,
applies them and stores the result. Rules live in the domain types; a service only decides the order
of calls. Errors from the domain and the stores come back as they are, so callers check them with
`errors.Is`.

Both services take a clock, `now func() time.Time`. Production code passes `time.Now`; tests pass a
function returning a fixed time. Every method passes its `ctx` to every store call.

## `BookingService`

```go
NewBookingService(bookings BookingStore, spaces SpaceStore, members MemberStore, now func() time.Time) *BookingService
```

### `Book(ctx, actor, spaceID, p) (Booking, error)`

Creates a booking of the space for `p.MemberID` on behalf of `actor`.

1. `GetSpace(spaceID)`
2. `NewBooking(actor, space, loc, p, now)`: all rules, permission first. See [Booking](booking.md)
3. `GetMember(p.MemberID)`: refuse with `ErrMemberDeleted` if deleted
4. `CreateBooking`: `ErrSlotTaken` if another booking holds the slot

`NewBooking` runs before the member is loaded, so a refused actor learns nothing about the member.

### `Cancel(ctx, actor, bookingID) (Booking, error)`

1. If `actor` is a member: `GetMember`, refuse with `ErrMemberDeleted` if deleted
2. `GetBooking(bookingID)`
3. `GetSpace(booking's space)`
4. `Booking.Cancel(actor, space, now)`
5. `SaveCancellation`: `ErrAlreadyCancelled` if another cancel won

### `Approve(ctx, actor, bookingID) (Booking, error)`

1. If `actor` is a member: `GetMember`, refuse with `ErrMemberDeleted` if deleted
2. `GetBooking(bookingID)`
3. `Booking.Approve(actor, now)`
4. `SaveApproval`: `ErrAlreadyCancelled` or `ErrNotPending` if another call won

`Cancel` and `Approve` return the booking as stored.

## `MemberService`

```go
NewMemberService(members MemberStore, bookings BookingStore, now func() time.Time) *MemberService
```

### `Delete(ctx, memberID) (Member, error)`

Marks the member deleted and cancels every booking of theirs that hasn't ended. The system cancels,
not a person, so no actor rules apply to the bookings.

1. `GetMember(memberID)`
2. `Member.Delete(now)`: `ErrMemberDeleted` if already deleted
3. `ListMemberBookingsEndingAfter(memberID, now)`
4. Cancel each one that hasn't ended or been cancelled
5. `SaveDeletion(member, cancelled)`: all or nothing; `ErrMemberDeleted` if another deletion won

Takes no actor yet: who may delete arrives with 3.7 and 4.6.

## Known gaps

- `Delete` lists, then saves, so a booking created in between isn't in its list. The Postgres
  `SaveDeletion` cancels it anyway, in the same transaction (2.6); the in-memory store doesn't.
- `Cancel` and `Approve` return `ErrBookingNotFound` before checking permission, so a member can
  tell which booking IDs exist within their operator. Revisit at 3.6.
