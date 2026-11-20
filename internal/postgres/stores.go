package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
)

// The compiler checks that Stores implements the three interfaces through
// the stores it embeds.
var (
	_ cowbook.BookingStore = Stores{}
	_ cowbook.SpaceStore   = Stores{}
	_ cowbook.MemberStore  = Stores{}
)

// Stores holds one tenant's three stores in one value. Each is embedded, a
// field with a type and no name, so its methods are promoted: Stores has
// CreateSpace, GetMember, ListBookings and the rest as if it declared them,
// and meets every interface the three meet between them, including ones
// declared by callers this package never sees. No two of them have a method
// with the same name; if they did, neither would be promoted.
type Stores struct {
	*SpaceStore
	*MemberStore
	*BookingStore
}

// NewStores returns the stores for tenant tenantID. Like the stores, it holds
// no connection.
func NewStores(pool *pgxpool.Pool, tenantID int64) Stores {
	return Stores{
		SpaceStore:   NewSpaceStore(pool, tenantID),
		MemberStore:  NewMemberStore(pool, tenantID),
		BookingStore: NewBookingStore(pool, tenantID),
	}
}
