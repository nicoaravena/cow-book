# REPL

`cowbook repl` is a command prompt over the services and the in-memory store. It is one
subcommand of the `cowbook` binary in `cmd/cowbook`, which grows each step into the full app. Every
step adds or changes commands here, so you can see the step working.

```sh
go run ./cmd/cowbook repl            # type commands; help lists them
```

When input is not a terminal (a pipe or a heredoc), the REPL runs in **script mode**: it echoes each command after its
prompt and copies blank lines and `#` comments, so the output reads like a session. A command that
fails prints `error: ...` and the session carries on. `quit` or the end of input ends it.

The prompt shows who is acting: `[nobody]>`, `[member 1]>`, `[frontdesk 7 1,2]>`, `[admin 1]>`.

## State

- **Clock.** Starts at Monday 16 November 2026, 08:00 UTC, so a session prints the same every run.
  The services read it, so moving it changes what counts as "the past" or a passed deadline.
- **IDs.** The REPL numbers spaces and members from 1, in the order they are added. Bookings get
  their IDs from the store.
- **Store.** In memory: everything is gone when the REPL ends.

## Commands

| Command | Does |
| --- | --- |
| `clock` | Shows the time |
| `clock set <YYYY-MM-DD> <HH:MM>` | Sets the time, in UTC |
| `clock add <duration>` | Moves the time on, e.g. `90m`, `2h30m`, `25h` |
| `space add <name> <kind> <capacity> <max-length> [hours] [timezone]` | Adds a space at location 1 |
| `member add <name> <email>` | Adds a member |
| `as member <id>` | Acts as that member |
| `as frontdesk <id> <loc,loc...>` | Acts as front desk staff assigned to those locations |
| `as admin <id>` | Acts as an admin |
| `book <spaceID> <YYYY-MM-DD> <HH:MM-HH:MM> [attendees] [for <memberID>]` | Books a space |
| `help` | Lists the commands |
| `quit` | Ends the session |

### `space add`

- `kind` is one of `hot_desk`, `private_office`, `meeting_room`, `phone_booth`.
- `max-length` is a Go duration: `2h`, `90m`.
- `hours` lists opening windows: `mon-fri/09:00-18:00,sat/10:00-14:00`. A day range runs forward,
  `mon` to `sun`. `24:00` closes at midnight. Default: `mon-fri/08:00-20:00`.
- `timezone` is an IANA name such as `Europe/Madrid`. Default: `UTC`. To give a timezone, give the
  hours too.
- Names are one word; use `Room-A`, not `Room A`.

### `book`

- The date and times are read on the wall clock **at the space**, in its timezone, and the answer
  shows them the same way. An end of `24:00` is midnight.
- `attendees` defaults to 1.
- A member books for themselves; `for` is optional. Staff must name the member with `for`, and the
  booking is pending until that member approves, by the deadline shown.

## Changes by step

| Step | Change |
| --- | --- |
| [1.9](../concepts/1.9-repl.md) | First version: `clock`, `space add`, `member add`, `as`, `book`, `help`, `quit` |
