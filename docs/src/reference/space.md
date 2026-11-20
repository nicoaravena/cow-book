# Space

`cowbook/space.go`. A bookable place at a location: a hot desk, private office, meeting room or
phone booth.

## `NewSpace(p SpaceParams, now time.Time) (Space, error)`

Validates `p` and returns an **available** space with `createdAt` and `updatedAt` set to `now` and
`ID` 0. The name is trimmed of surrounding whitespace before it's checked and stored.

`SpaceParams` fields: `LocationID`, `Name`, `Kind`, `Capacity`, `MaxBookingLength`, `Hours`.

Rules, in the order checked. The first one broken is returned:

| Rule | Error |
| --- | --- |
| `LocationID` > 0 | `ErrInvalidLocation` |
| `Name` not blank after trimming | `ErrSpaceNameRequired` |
| `Kind` is one of the four kinds below | `ErrInvalidSpaceKind` |
| `Capacity` >= 1 | `ErrInvalidCapacity` |
| 0 < `MaxBookingLength` <= 24h | `ErrInvalidMaxBookingLength` |
| `Hours` open at least once a week | `ErrNoOpeningHours` |

The 24-hour cap exists because a booking must fit inside one opening window, which is inside one
day, so a longer limit means nothing.

## `RestoreSpace(r SpaceRecord) (Space, error)`

Rebuilds a stored space, for stores (2.4b). It calls `NewSpace(r.SpaceParams, r.CreatedAt)`, so
the same rules apply in the same order and the first broken one is returned unchanged. Then it
sets `ID`, `available` and `updatedAt` from the record.

`SpaceRecord` fields: `ID`, the embedded `SpaceParams` (its fields are promoted, so `r.Name`
works), `Available`, `CreatedAt`, `UpdatedAt`.

## `SpaceKind`

A `string` type. Values, as they will be stored and sent over the API:

| Constant | Value |
| --- | --- |
| `KindHotDesk` | `"hot_desk"` |
| `KindPrivateOffice` | `"private_office"` |
| `KindMeetingRoom` | `"meeting_room"` |
| `KindPhoneBooth` | `"phone_booth"` |

## Accessors

Read-only, for the REPL (1.10) and stores outside the package (2.4b):

| Method | Returns |
| --- | --- |
| `Name()` | The trimmed name |
| `Kind()` | The `SpaceKind` |
| `Capacity()` | Most attendees per booking |
| `MaxBookingLength()` | Longest booking allowed |
| `LocationID()` | The location's ID |
| `Hours()` | The `WeeklyHours` |
| `Available()` | Whether it takes bookings |
| `CreatedAt()`, `UpdatedAt()` | When it was created and last changed |

## Fields

| Field | Exported | Notes |
| --- | --- | --- |
| `ID` | yes | Set by the store |
| `locationID` | no | Decides what front desk staff may do with its bookings |
| `name`, `kind`, `capacity`, `maxBookingLength` | no | |
| `hours` | no | `WeeklyHours`, see [Opening hours](opening-hours.md) |
| `available` | no | `true` from `NewSpace`. `NewBooking` refuses an unavailable space. Only `RestoreSpace` can set it to `false` yet |
| `createdAt`, `updatedAt` | no | |

A `Space` doesn't know its timezone. Its location does, so `SpaceStore.GetSpace` returns the
`*time.Location` alongside it.

## Not yet

- No create or update service. `CreateSpace` on `memstore.Store` and `postgres.SpaceStore` stores
  spaces built by the caller (memstore's since 2.7); space management arrives in 3.7.
