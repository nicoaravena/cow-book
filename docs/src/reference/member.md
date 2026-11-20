# Member

`cowbook/member.go`. A person who books spaces at one operator. Members are soft-deleted: the record
stays, so bookings and history still point at a real member.

## `NewMember(p MemberParams, now time.Time) (Member, error)`

Returns a member with `ID` 0 and `createdAt`, `updatedAt` set to `now`.

`MemberParams` fields: `Name`, `Email`.

- The name is trimmed.
- The email is trimmed and lowercased, since it must be unique per operator: `Ana@x.com` and
  `ana@x.com` are the same member. Uniqueness itself is the store's job: `CreateMember` returns
  `ErrEmailTaken` for an email a member who isn't deleted has (2.7), which Postgres checks with a
  partial unique index (2.2).

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
| `CreatedAt()`, `UpdatedAt()` | When it was created and last changed (2.6) |
| `DeletedAt()` | When it was deleted; zero if not (2.6) |

## `RestoreMember(r MemberRecord) (Member, error)`

Rebuilds a stored member (2.6). `MemberRecord` embeds `MemberParams` and adds `ID`, `CreatedAt`,
`UpdatedAt` and `DeletedAt`, a zero `DeletedAt` meaning not deleted. It goes through `NewMember`,
as `RestoreSpace` goes through `NewSpace`, so a stored name or email that breaks a rule returns
that rule's error, and the name and email are trimmed and lowercased again.

## Effects of deletion

| Where | Effect |
| --- | --- |
| `MemberService.Delete` | Cancels every booking of theirs that hasn't ended, pending and in-progress ones included |
| `BookingService.Book` | Refuses to book for them: `ErrMemberDeleted`. In Postgres, `CreateBooking` also refuses a member deleted after the service's check (2.6) |
| `BookingService.Cancel`, `Approve` | Refuses them as the actor: `ErrMemberDeleted`. Staff can still cancel their bookings |
| `MemberStore.GetMember` | Still returns them, so callers can tell deleted from missing |

## Not yet

- Login must refuse deleted members (4.2).
- A deleted member's email can register again: the unique index (2.2) and memstore's check (2.7)
  ignore deleted members.
- Name and email are kept after deletion. Erasing them on request is not covered.
