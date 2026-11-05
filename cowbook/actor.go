package cowbook

import "slices"

// role says what an Actor is allowed to do. It starts at iota + 1 so that the
// zero value is no role at all, and an Actor{} can do nothing.
type role int

const (
	roleMember role = iota + 1
	roleFrontDesk
	roleAdmin
)

// Actor is whoever is asking for something to happen. Platform support has no
// Actor on purpose: it must not act on bookings.
type Actor struct {
	id        int64
	role      role
	locations []int64 // front desk only
}

// MemberActor returns the Actor for the member with this ID.
func MemberActor(id int64) Actor {
	return Actor{id: id, role: roleMember}
}

// AdminActor returns the Actor for an admin, who covers the whole operator.
func AdminActor(id int64) Actor {
	return Actor{id: id, role: roleAdmin}
}

// FrontDeskActor returns the Actor for front desk staff assigned to locations.
// It keeps its own copy, so later changes to the caller's slice don't change
// what the Actor may do.
func FrontDeskActor(id int64, locations []int64) Actor {
	return Actor{id: id, role: roleFrontDesk, locations: slices.Clone(locations)}
}

// canCancel reports whether a may cancel b, which is at locationID. The ID is
// compared only for members: members and staff are separate tables, so staff 3
// is not member 3.
func (a Actor) canCancel(b Booking, locationID int64) bool {
	switch a.role {
	case roleMember:
		return a.id == b.memberID
	case roleFrontDesk:
		return slices.Contains(a.locations, locationID)
	case roleAdmin:
		return true
	}
	return false
}
