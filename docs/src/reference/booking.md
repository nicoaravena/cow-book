# Booking

`cowbook/booking.go`. A member's reservation of a space for a period of time. Overlaps with other
bookings are not checked here: the store decides those (see [Stores](stores.md)).

## States

There is no status field. The state comes from which timestamps are set (zero means not set):

| `approvalDeadline` | `approvedAt` | `cancelledAt` | State |
| --- | --- | --- | --- |
| zero | zero | zero | **Confirmed**: the member booked it |
| set | zero | zero | **Pending** until the deadline, then **expired** |
| set | set | zero | **Approved** |
| any | any | set | **Cancelled** |

Pending and expired bookings both hold their slot. An expired one is cancelled lazily when a new
booking needs the slot (2.5; the in-memory store doesn't do this).

```
              NewBooking by member           NewBooking by staff
                     |                              |
                     v                              v
                 Confirmed                       Pending ---- deadline passes ---> Expired
                     |                          |       \
                     |                   Approve |        \ Cancel (= decline)
                     |                          v         \
                     |                       Approved      \
                     |                          |           |
                     +------ Cancel ------------+-----------+--> Cancelled
```

## `(Booking) State(now time.Time) BookingState`

Names the state above at `now`, for showing to people (1.10). The rules read the timestamps, not
this. `BookingState` is a `string` type:

| Constant | Value | When |
| --- | --- | --- |
| `StateConfirmed` | `"confirmed"` | The member booked it, or approved it |
| `StatePending` | `"pending"` | Made by staff, not approved, `now` before the deadline |
| `StateExpired` | `"expired"` | Made by staff, not approved, `now` at or after the deadline |
| `StateCancelled` | `"cancelled"` | Cancelled or declined, whatever else is set |

An expired booking is the one `Approve` refuses with `ErrApprovalExpired`: both use
`!now.Before(deadline)`.

## `NewBooking(actor, space, loc, p, now) (Booking, error)`

Validates `p` against `space` and returns a booking made by `actor`, with `ID` 0. `loc` is the
timezone of the space's location and must not be nil.

`BookingParams` fields: `MemberID`, `Attendees`, `Start`, `End`.

Rules, in the order checked. The first one broken is returned:

| Rule | Error |
| --- | --- |
| `actor` may act for `p.MemberID` at the space's location (see [Actor](actor.md)) | `ErrBookNotAllowed` |
| `p.MemberID` > 0 | `ErrInvalidMember` |
| `space.ID` > 0 (stored) | `ErrInvalidSpace` |
| Space is available | `ErrSpaceUnavailable` |
| `Attendees` >= 1 | `ErrInvalidAttendees` |
| `Attendees` <= capacity | `ErrOverCapacity`, wrapped: `9 attendees, capacity 8` |
| `Start` before `End` | `ErrInvalidBookingTime` |
| `Start` not before `now` | `ErrBookingInPast` |
| `End - Start` <= max booking length (real elapsed time) | `ErrBookingTooLong`, wrapped: `5h0m0s, limit 4h0m0s` |
| Inside one opening window, see [Opening hours](opening-hours.md) | `ErrOutsideOpeningHours` |

If `actor` is not a member (front desk or admin), the booking is **pending**: `bookedBy` is the
staff ID and `approvalDeadline` is `now + 24h`, or `Start` if that is sooner.

The end is exclusive: a booking ending at 10:00 and one starting at 10:00 don't overlap.

## `(*Booking) Cancel(actor Actor, space Space, now time.Time) error`

Marks the booking cancelled, which frees its slot. `space` must be the booking's own space; its
location decides what front desk staff may do. Declining a pending booking is cancelling it.

| Rule | Error |
| --- | --- |
| `space.ID` is the booking's space, and `actor` may act for the booking's member at its location | `ErrCancelNotAllowed` |
| Not already cancelled | `ErrAlreadyCancelled` |
| Not ended (`now` before `End`) | `ErrBookingEnded` |

A booking in progress can be cancelled, which frees the rest of the slot. An ended one stays as it
is, to keep the history true.

## `(*Booking) Approve(actor Actor, now time.Time) error`

Records the member's approval of a booking staff made for them.

| Rule | Error |
| --- | --- |
| `actor` is the member the booking is for (staff can't approve) | `ErrApproveNotAllowed` |
| Not cancelled | `ErrAlreadyCancelled` |
| Made by staff and not yet approved | `ErrNotPending` |
| `now` before the approval deadline | `ErrApprovalExpired` |

Both `Cancel` and `Approve` check permission first and leave the booking unchanged on error.

## Accessors

Read-only, for stores and the REPL outside the package:

| Method | Returns |
| --- | --- |
| `SpaceID()` | The booked space's ID |
| `MemberID()` | The member the booking is for |
| `Attendees()` | How many people it is for (1.10) |
| `Start()`, `End()` | The period; `End` is exclusive |
| `Cancelled()` | Whether `cancelledAt` is set |
| `Approved()` | Whether `approvedAt` is set. `false` for a booking that never needed approval |
| `ApprovalDeadline()` | When a staff-made booking must be approved by; zero for a member's own booking (1.9) |

Not exposed yet: `bookedBy`, the other timestamps, and a way to rebuild a booking
from a database row (2.5).

## Known gaps

- A staff booking that starts right away gets a deadline equal to its start, which is already now,
  so it can never be approved. It holds the slot until lazy expiry cancels it.
- Staff can hold a slot for up to 24 hours with a booking the member never approves.
