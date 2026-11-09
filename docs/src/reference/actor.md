# Actor

`cowbook/actor.go`. Whoever is asking for something to happen. Rules that depend on who asks take
an `Actor`.

Platform support has no `Actor` on purpose: it must not act on bookings.

## Constructors

The fields are unexported, so these are the only way to make one. An `Actor{}` has no role and is
refused everything.

| Constructor | Role | Covers |
| --- | --- | --- |
| `MemberActor(id)` | member | Only themselves: `id` is a member ID |
| `FrontDeskActor(id, locations)` | front desk | Any member, at the listed locations only. Keeps its own copy of `locations` |
| `AdminActor(id)` | admin | Any member, at every location of the operator |

For staff, `id` is a staff ID. Members and staff are separate tables, so staff 3 is not member 3:
IDs are compared only for members.

## What each role may do

| Action | Member | Front desk | Admin |
| --- | --- | --- | --- |
| Book for a member (`NewBooking`) | Themselves | Any member, at their locations | Any member |
| Cancel a booking (`Booking.Cancel`) | Their own | Any, at their locations | Any |
| Approve a booking (`Booking.Approve`) | Their own, if staff made it | No | No |
| Booking made is | Confirmed | Pending | Pending |
| Refused when deleted (services) | Yes, `ErrMemberDeleted` | n/a | n/a |

Front desk staff can cancel a deleted member's bookings at their locations; admins anywhere.

Who may delete a member, and who may manage spaces or staff, isn't checked yet (3.7, 4.6).
