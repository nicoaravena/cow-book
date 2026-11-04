package cowbook

import (
	"errors"
	"strings"
	"time"
)

// maxBookingLengthLimit is the longest limit a space can set. A booking
// must fit inside one day's opening hours, so a longer limit means nothing.
const maxBookingLengthLimit = 24 * time.Hour

// Errors returned by NewSpace. Callers compare them with errors.Is.
var (
	ErrInvalidLocation         = errors.New("space needs a valid location")
	ErrSpaceNameRequired       = errors.New("space name is required")
	ErrInvalidSpaceKind        = errors.New("space kind is not recognized")
	ErrInvalidCapacity         = errors.New("space capacity must be at least 1")
	ErrInvalidMaxBookingLength = errors.New("max booking length must be more than 0 and at most 24 hours")
	ErrNoOpeningHours          = errors.New("space needs opening hours")
)

// SpaceKind says what type of space is being rented out.
type SpaceKind string

const (
	KindHotDesk       SpaceKind = "hot_desk"
	KindPrivateOffice SpaceKind = "private_office"
	KindMeetingRoom   SpaceKind = "meeting_room"
	KindPhoneBooth    SpaceKind = "phone_booth"
)

func (k SpaceKind) valid() bool {
	switch k {
	case KindHotDesk, KindPrivateOffice, KindMeetingRoom, KindPhoneBooth:
		return true
	}
	return false
}

// Space is a bookable place at a location.
type Space struct {
	ID               int64
	locationID       int64
	name             string
	kind             SpaceKind
	capacity         int
	maxBookingLength time.Duration
	hours            WeeklyHours
	available        bool
	createdAt        time.Time
	updatedAt        time.Time
}

// SpaceParams holds the values needed to create a Space.
type SpaceParams struct {
	LocationID       int64
	Name             string
	Kind             SpaceKind
	Capacity         int
	MaxBookingLength time.Duration
	Hours            WeeklyHours
}

// NewSpace validates p and returns an available Space. The ID stays 0
// until the space is stored. It returns the first rule that p breaks.
func NewSpace(p SpaceParams, now time.Time) (Space, error) {
	name := strings.TrimSpace(p.Name)

	switch {
	case p.LocationID <= 0:
		return Space{}, ErrInvalidLocation
	case name == "":
		return Space{}, ErrSpaceNameRequired
	case !p.Kind.valid():
		return Space{}, ErrInvalidSpaceKind
	case p.Capacity < 1:
		return Space{}, ErrInvalidCapacity
	case p.MaxBookingLength <= 0 || p.MaxBookingLength > maxBookingLengthLimit:
		return Space{}, ErrInvalidMaxBookingLength
	case p.Hours.closedAllWeek():
		return Space{}, ErrNoOpeningHours
	}

	return Space{
		locationID:       p.LocationID,
		name:             name,
		kind:             p.Kind,
		capacity:         p.Capacity,
		maxBookingLength: p.MaxBookingLength,
		hours:            p.Hours,
		available:        true,
		createdAt:        now,
		updatedAt:        now,
	}, nil
}
