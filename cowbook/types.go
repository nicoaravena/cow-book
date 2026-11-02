package cowbook

import "time"

type Booking struct {
	ID        uint64
	spaceID   uint64
	memberID  uint64
	start     time.Time
	end       time.Time
	createdAt time.Time
	updatedAt time.Time
	deletedAt time.Time
}

type Member struct {
	ID        uint64
	name      string
	createdAt time.Time
	updatedAt time.Time
	deletedAt time.Time
}
