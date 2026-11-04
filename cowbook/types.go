package cowbook

import "time"

type Member struct {
	ID        uint64
	name      string
	createdAt time.Time
	updatedAt time.Time
	deletedAt time.Time
}
