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

// The accessors below let code outside this package read a space without
// being able to change it.

// Name returns the space's name.
func (s Space) Name() string { return s.name }

// Kind returns what type of space it is.
func (s Space) Kind() SpaceKind { return s.kind }

// Capacity returns how many people the space holds.
func (s Space) Capacity() int { return s.capacity }

// MaxBookingLength returns the longest booking the space takes.
func (s Space) MaxBookingLength() time.Duration { return s.maxBookingLength }

// LocationID returns the ID of the location the space is at.
func (s Space) LocationID() int64 { return s.locationID }

// Hours returns the space's weekly opening hours.
func (s Space) Hours() WeeklyHours { return s.hours }

// Available reports whether the space takes bookings.
func (s Space) Available() bool { return s.available }

// CreatedAt returns when the space was created.
func (s Space) CreatedAt() time.Time { return s.createdAt }

// UpdatedAt returns when the space last changed.
func (s Space) UpdatedAt() time.Time { return s.updatedAt }

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

// SpaceRecord holds a stored space's values: the ones NewSpace takes, plus
// those a store keeps. SpaceParams is embedded, so its fields read as the
// record's own: r.Name is r.SpaceParams.Name.
type SpaceRecord struct {
	ID int64
	SpaceParams
	Available bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RestoreSpace rebuilds a stored space. It checks r with NewSpace's rules,
// so a row that breaks one fails here instead of reaching a booking.
func RestoreSpace(r SpaceRecord) (Space, error) {
	s, err := NewSpace(r.SpaceParams, r.CreatedAt)
	if err != nil {
		return Space{}, err
	}
	s.ID = r.ID
	s.available = r.Available
	s.updatedAt = r.UpdatedAt
	return s, nil
}
