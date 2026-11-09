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

## `SpaceKind`

A `string` type. Values, as they will be stored and sent over the API:

| Constant | Value |
| --- | --- |
| `KindHotDesk` | `"hot_desk"` |
| `KindPrivateOffice` | `"private_office"` |
| `KindMeetingRoom` | `"meeting_room"` |
| `KindPhoneBooth` | `"phone_booth"` |

## Fields

| Field | Exported | Notes |
| --- | --- | --- |
| `ID` | yes | Set by the store |
| `locationID` | no | Decides what front desk staff may do with its bookings |
| `name`, `kind`, `capacity`, `maxBookingLength` | no | |
| `hours` | no | `WeeklyHours`, see [Opening hours](opening-hours.md) |
| `available` | no | `true` from `NewSpace`. `NewBooking` refuses an unavailable space. Nothing sets it to `false` yet |
| `createdAt`, `updatedAt` | no | |

A `Space` doesn't know its timezone. Its location does, so `SpaceStore.GetSpace` returns the
`*time.Location` alongside it.

## Not yet

- No accessors. `internal/postgres` will need them, and a way to rebuild a `Space` from a row (2.4).
- No create or update service. `memstore.AddSpace` seeds spaces in tests; space management arrives
  in 3.7.
