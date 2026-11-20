# REPL

`cowbook repl` is a command prompt over the services and a store, in memory or in Postgres. It is
one subcommand of the `cowbook` binary in `cmd/cowbook`, which grows each step into the full app.
Every step adds or changes commands here, so you can see the step working.

```sh
go run ./cmd/cowbook repl                                     # in memory; help lists the commands
go run ./cmd/cowbook repl -store=postgres -tenant=north-hub   # one tenant in DATABASE_URL (2.7)
```

## Flags

| Flag | Default | Means |
| --- | --- | --- |
| `-store` | `memory` | `memory`: gone when the REPL ends. `postgres`: the tenant's schema in `DATABASE_URL`, kept across runs |
| `-tenant` | none | The operator's slug, required with `-store=postgres` and refused without it. The tenant must exist and be `active` |
| `-h` | | Prints the flags |

| Exit code | Means |
| --- | --- |
| `0` | The session ended, or `-h` |
| `1` | `DATABASE_URL` unset, Postgres unreachable, tenant unknown or not active, or reading stdin failed |
| `2` | A bad flag or argument; the problem and the flags go to stderr |

When input is not a terminal (a pipe or a heredoc), the REPL runs in **script mode**: it echoes each command after its
prompt and copies blank lines and `#` comments, so the output reads like a session. A command that
fails prints `error: ...` and the session carries on. `quit` or the end of input ends it.

The prompt shows who is acting: `[nobody]>`, `[member 1]>`, `[frontdesk 7 1,2]>`, `[admin 1]>`.

## State

- **Clock.** Starts at Monday 16 November 2026, 08:00 UTC, so a session prints the same every run,
  with either store. The services read it, so moving it changes what counts as "the past" or a
  passed deadline. With `-store=postgres` the data stays but the clock starts over each run.
- **IDs.** The store hands them out (2.7): from 1 in memory, and from the tenant's identity
  columns in Postgres, which skip numbers after a refused insert.
- **Locations.** Each timezone stands for one location until 3.7. The first space in a zone adds
  its location, named after the zone. In memory, UTC is location 1 if the first space is in UTC.
- **Store.** The session reads and writes through `repl.Store`; see [Stores](stores.md#replstore).

## Commands

| Command | Does |
| --- | --- |
| `clock` | Shows the time |
| `clock set <YYYY-MM-DD> <HH:MM>` | Sets the time, in UTC |
| `clock add <duration>` | Moves the time on, e.g. `90m`, `2h30m`, `25h` |
| `space add <name> <kind> <capacity> <max-length> [hours] [timezone]` | Adds a space at its timezone's location |
| `space list` | Lists spaces with kind, capacity, max length and timezone |
| `member add <name> <email>` | Adds a member; refused if a member who isn't deleted has the email |
| `member delete <id>` | Deletes a member and cancels their bookings that haven't ended |
| `member list` | Lists members, `active` or `deleted` |
| `as member <id>` | Acts as that member |
| `as frontdesk <id> <loc,loc...>` | Acts as front desk staff assigned to those locations |
| `as admin <id>` | Acts as an admin |
| `book <spaceID> <YYYY-MM-DD> <HH:MM-HH:MM> [attendees] [for <memberID>]` | Books a space |
| `cancel <bookingID>` | Cancels a booking, or declines one staff made for you |
| `approve <bookingID>` | Approves a booking staff made for you |
| `booking list` | Lists bookings with their state at the session's time |
| `help` | Lists the commands |
| `quit` | Ends the session |

### `space add`

- `kind` is one of `hot_desk`, `private_office`, `meeting_room`, `phone_booth`.
- `max-length` is a Go duration: `2h`, `90m`.
- `hours` lists opening windows: `mon-fri/09:00-18:00,sat/10:00-14:00`. A day range runs forward,
  `mon` to `sun`. `24:00` closes at midnight. Default: `mon-fri/08:00-20:00`.
- `timezone` is an IANA name such as `Europe/Madrid`. Default: `UTC`. To give a timezone, give the
  hours too. The timezone picks the location (2.7), so front desk staff `as frontdesk 7 2` work at
  the spaces in the second zone added.
- Names are one word; use `Room-A`, not `Room A`.

### `book`

- The date and times are read on the wall clock **at the space**, in its timezone, and the answer
  shows them the same way. An end of `24:00` is midnight.
- `attendees` defaults to 1.
- A member books for themselves; `for` is optional. Staff must name the member with `for`, and the
  booking is pending until that member approves, by the deadline shown.

### `cancel`, `approve`, `member delete`

- `cancel` and `approve` need someone acting (`as`). The rules are the domain's: a member cancels
  their own bookings, staff those of members they act for; only the member approves, before the
  deadline.
- `member delete` needs no actor yet, since the service doesn't check who deletes (M4).

### Lists

Columns line up with `text/tabwriter`. Rows are sorted by ID. An empty list prints `no spaces`,
`no members` or `no bookings`.

- `booking list` shows each booking's time on the wall clock at its space, like `book`, and its
  state: `confirmed`, `pending`, `expired` (deadline passed unapproved; still holds its slot) or
  `cancelled`. See [Booking](booking.md#booking-statenow-timetime-bookingstate).
- `member list` includes deleted members.

## Changes by step

| Step | Change |
| --- | --- |
| [1.9](../concepts/1.9-repl.md) | First version: `clock`, `space add`, `member add`, `as`, `book`, `help`, `quit` |
| [1.10](../concepts/1.10-repl-catches-up.md) | `cancel`, `approve`, `member delete`, `space list`, `member list`, `booking list` |
| [2.7](../concepts/2.7-repl-on-postgres.md) | `-store` and `-tenant` flags; IDs from the store; a location per timezone; `member add` refuses a taken email |
