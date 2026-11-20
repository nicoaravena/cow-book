# Errors

Every error `cowbook` returns is a sentinel declared in the package. Compare with `errors.Is`,
never `==`: errors marked **wrapped** come with details added by `fmt.Errorf("%w: ...")`, and others
may later.

The **Kind** column is a proposed grouping for the HTTP layer, which will map each kind to a status
in 3.5. It isn't in the code, and 3.5 may change it:

- **invalid**: the request breaks a rule. Fixing the input fixes it.
- **forbidden**: the actor may not do this.
- **conflict**: the state changed or doesn't allow it; often a lost race.
- **not found**

## Space and opening hours

| Error | Kind | From |
| --- | --- | --- |
| `ErrInvalidLocation` | invalid | `NewSpace`; `CreateSpace` for an unknown location |
| `ErrSpaceNameRequired` | invalid | `NewSpace` |
| `ErrInvalidSpaceKind` | invalid | `NewSpace` |
| `ErrInvalidCapacity` | invalid | `NewSpace` |
| `ErrInvalidMaxBookingLength` | invalid | `NewSpace` |
| `ErrNoOpeningHours` | invalid | `NewSpace` |
| `ErrInvalidClockTime` | invalid | `NewClockTime` |
| `ErrInvalidOpeningWindow` | invalid | `NewWeeklyHours` |
| `ErrOverlappingWindows` | invalid | `NewWeeklyHours` |

## Booking

| Error | Kind | From |
| --- | --- | --- |
| `ErrBookNotAllowed` | forbidden | `NewBooking` |
| `ErrInvalidMember` | invalid | `NewBooking`, `RestoreBooking` |
| `ErrInvalidSpace` | invalid | `NewBooking`, `RestoreBooking` |
| `ErrSpaceUnavailable` | conflict | `NewBooking` |
| `ErrInvalidAttendees` | invalid | `NewBooking`, `RestoreBooking` |
| `ErrOverCapacity` (wrapped) | invalid | `NewBooking` |
| `ErrInvalidBookingTime` | invalid | `NewBooking`, `RestoreBooking` |
| `ErrBookingInPast` | invalid | `NewBooking` |
| `ErrBookingTooLong` (wrapped) | invalid | `NewBooking` |
| `ErrOutsideOpeningHours` | invalid | `NewBooking` |
| `ErrCancelNotAllowed` | forbidden | `Booking.Cancel` |
| `ErrAlreadyCancelled` | conflict | `Booking.Cancel`, `Booking.Approve`, `SaveCancellation`, `SaveApproval` |
| `ErrBookingEnded` | conflict | `Booking.Cancel` |
| `ErrApproveNotAllowed` | forbidden | `Booking.Approve` |
| `ErrNotPending` | conflict | `Booking.Approve`, `SaveApproval` |
| `ErrApprovalExpired` | conflict | `Booking.Approve`, `postgres.BookingStore.SaveApproval` |

## Member

| Error | Kind | From |
| --- | --- | --- |
| `ErrMemberNameRequired` | invalid | `NewMember` |
| `ErrInvalidMemberEmail` | invalid | `NewMember` |
| `ErrMemberDeleted` | conflict | `Member.Delete`, `SaveDeletion`, the `BookingService` methods for a deleted member, and `postgres.BookingStore.CreateBooking` for a member deleted meanwhile (2.6) |

## Stores

Declared in `cowbook/service.go`, returned by store implementations and passed on by the services.

| Error | Kind | From |
| --- | --- | --- |
| `ErrSlotTaken` | conflict | `CreateBooking`; in Postgres, `23P01` from `bookings_no_overlap` |
| `ErrBookingNotFound` | not found | `GetBooking`, `SaveCancellation`, `SaveApproval` |
| `ErrSpaceNotFound` | not found | `GetSpace`; Postgres `CreateBooking` for a space deleted meanwhile |
| `ErrMemberNotFound` | not found | `GetMember`, `SaveDeletion`; Postgres `CreateBooking` for a member removed meanwhile |
| `ErrEmailTaken` | conflict | `CreateMember` (2.7); in Postgres, `23505` from `members_email_key` |

`internal/postgres` declares one of its own: `ErrTenantNotFound` (not found), from `LookupTenant`
when no tenant has the slug (2.7). It is wrapped with the slug.

Stores may also return `ctx.Err()` (`context.Canceled`, `context.DeadlineExceeded`) when the
caller's context is done, and, in M2, database errors that aren't any of the above.

## Example

```go
_, err := svc.Book(ctx, actor, spaceID, p)
switch {
case err == nil:
	// booked
case errors.Is(err, cowbook.ErrSlotTaken):
	// someone else got it
case errors.Is(err, cowbook.ErrOverCapacity):
	// err.Error() also says how many attendees and the capacity
}
```
