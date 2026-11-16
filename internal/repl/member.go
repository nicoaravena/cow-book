package repl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"nicoaravena/cow-book-app/cowbook"
)

func (s *Session) member(_ context.Context, args []string) (string, error) {
	if len(args) != 3 || args[0] != "add" {
		return "", errUsage
	}
	m, err := cowbook.NewMember(cowbook.MemberParams{Name: args[1], Email: args[2]}, s.clock.Now())
	if err != nil {
		return "", err
	}
	s.lastMemberID++
	m.ID = s.lastMemberID
	s.store.AddMember(m)
	return fmt.Sprintf("ok member #%d", m.ID), nil
}

// as sets who the next commands act as. It checks only the words: whether
// that member exists is the services' business, on the next booking.
func (s *Session) as(_ context.Context, args []string) (string, error) {
	if len(args) < 2 {
		return "", errUsage
	}
	id, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil || id <= 0 {
		return "", errUsage
	}

	switch {
	case args[0] == "member" && len(args) == 2:
		s.actor, s.memberID = cowbook.MemberActor(id), id
	case args[0] == "admin" && len(args) == 2:
		s.actor, s.memberID = cowbook.AdminActor(id), 0
	case args[0] == "frontdesk" && len(args) == 3:
		locations, err := parseIDs(args[2])
		if err != nil {
			return "", errUsage
		}
		s.actor, s.memberID = cowbook.FrontDeskActor(id, locations), 0
	default:
		return "", errUsage
	}
	s.who = strings.Join(args, " ")
	return "ok acting as " + s.who, nil
}

// parseIDs reads a comma-separated list of IDs, such as 1,2.
func parseIDs(s string) ([]int64, error) {
	var ids []int64
	for part := range strings.SplitSeq(s, ",") {
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, errors.New("not an ID list")
		}
		ids = append(ids, id)
	}
	return ids, nil
}
