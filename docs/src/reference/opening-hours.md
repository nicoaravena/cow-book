# Opening hours

`cowbook/hours.go`. When a space is open during a normal week, as wall-clock times with no date or
timezone. The timezone comes from the space's location when a booking is checked.

## `ClockTime`

A time of day to the minute. 24:00 is valid and means the very end of the day.

| Function | Does | Error |
| --- | --- | --- |
| `NewClockTime(hour, minute)` | Returns `hour:minute` | `ErrInvalidClockTime` unless 00:00 <= time <= 24:00 and 0 <= minute <= 59 |
| `Minutes()` | Minutes since midnight, 0 to 1440, as `opening_windows` stores them (2.4b) | - |

## `OpeningWindow`

One open period on one weekday. Exported fields: `Day time.Weekday`, `Open ClockTime`,
`Close ClockTime`. Validated by `NewWeeklyHours`, not on its own.

## `WeeklyHours`

The zero value is closed all week. Internally, windows are kept per weekday, sorted by opening time.

### `NewWeeklyHours(windows []OpeningWindow) (WeeklyHours, error)`

A day with no windows is closed. The caller's slice is not kept or changed.

| Rule | Error |
| --- | --- |
| Each window's `Day` is `time.Sunday` to `time.Saturday` and `Open` < `Close` | `ErrInvalidOpeningWindow` |
| Windows on the same day neither overlap nor touch | `ErrOverlappingWindows` |

Touching windows (09-12 plus 12-17) are refused because a booking must fit in one window, so
11-13 would be refused for no visible reason. Write 09-17 instead.

### `Windows() []OpeningWindow`

Every window, Sunday first and each day by opening time; empty when closed all week. The slice is
new, so changing it doesn't change the hours. Stores write one row per window (2.4b).

### `Contains(start, end time.Time, loc *time.Location) bool`

Reports whether the period from `start` to `end` fits inside **one** opening window, read on the
local clock of `loc`. `loc` must not be nil.

- `false` unless `start` is before `end`.
- The weekday and clock times come from `start` and `end` converted to `loc`. The location the
  inputs carry doesn't matter.
- The period must stay on one local date. A period ending exactly at midnight counts as ending at
  24:00 on the earlier day, including where the next day starts at 01:00 after a clock change.
- Seconds count: a period ending at 17:00:30 doesn't fit a window closing at 17:00.

**Known gap.** Only the two ends are checked. If the clock goes back during the period and a window
opens or closes inside the repeated hour, the period can pass while briefly outside the window. See
Risks in the [Roadmap](../roadmap.md). A test pins this behavior.

## Errors

| Error | From |
| --- | --- |
| `ErrInvalidClockTime` | `NewClockTime` |
| `ErrInvalidOpeningWindow` | `NewWeeklyHours` |
| `ErrOverlappingWindows` | `NewWeeklyHours` |
