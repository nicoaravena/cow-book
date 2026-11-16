# Member

`cowbook/member.go`. A person who books spaces at one operator. Members are soft-deleted: the record
stays, so bookings and history still point at a real member.

## `NewMember(p MemberParams, now time.Time) (Member, error)`

Returns a member with `ID` 0 and `createdAt`, `updatedAt` set to `now`.

`MemberParams` fields: `Name`, `Email`.

- The name is trimmed.
- The email is trimmed and lowercased, since it must be unique per operator: `Ana@x.com` and
  `ana@x.com` are the same member. Uniqueness itself is the database's job (2.2).

| Rule | Error |
| --- | --- |
| `Name` not blank after trimming | `ErrMemberNameRequired` |
| `Email` is a bare address (`ana@x.com`, not `Ana <ana@x.com>`) | `ErrInvalidMemberEmail` |

## `(*Member) Delete(now time.Time) error`

Sets `deletedAt` and `updatedAt` to `now`. Returns `ErrMemberDeleted` if already deleted, leaving
the member unchanged.

This only changes the member. Cancelling their bookings is `MemberService.Delete`'s job; see
[Services](services.md).

## Accessors

| Method | Returns |
| --- | --- |
| `Name()` | The trimmed name (1.10) |
| `Email()` | The lowercased email (1.10) |
| `Deleted()` | Whether `deletedAt` is set |

## Effects of deletion

| Where | Effect |
| --- | --- |
| `MemberService.Delete` | Cancels every booking of theirs that hasn't ended, pending and in-progress ones included |
| `BookingService.Book` | Refuses to book for them: `ErrMemberDeleted` |
| `BookingService.Cancel`, `Approve` | Refuses them as the actor: `ErrMemberDeleted`. Staff can still cancel their bookings |
| `MemberStore.GetMember` | Still returns them, so callers can tell deleted from missing |

## Not yet

- Login must refuse deleted members (4.2).
- Whether a deleted member's email can register again is decided by the unique index in 2.2.
- Name and email are kept after deletion. Erasing them on request is not covered.
