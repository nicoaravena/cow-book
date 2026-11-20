package postgres

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestIntervalToDuration(t *testing.T) {
	tests := []struct {
		name    string
		in      pgtype.Interval
		want    time.Duration
		wantErr bool
	}{
		{"microseconds", pgtype.Interval{Microseconds: (90 * time.Minute).Microseconds(), Valid: true}, 90 * time.Minute, false},
		// Typed as '1 day' in psql, the check constraint allows it as 24 hours.
		{"days", pgtype.Interval{Days: 1, Valid: true}, 24 * time.Hour, false},
		{"months", pgtype.Interval{Months: 1, Valid: true}, 0, true},
		{"null", pgtype.Interval{}, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := intervalToDuration(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("intervalToDuration(%+v) error = %v, want error %t", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("intervalToDuration(%+v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestDurationToIntervalRoundTrip(t *testing.T) {
	d := 2*time.Hour + 30*time.Minute
	got, err := intervalToDuration(durationToInterval(d))
	if err != nil || got != d {
		t.Errorf("intervalToDuration(durationToInterval(%v)) = %v, %v; want %v, nil", d, got, err, d)
	}
}
