package github

import (
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"strings"
)

// TypedUID is a custom TypedUID implementation for GitHub repository sources.
// We need it to read repository owner and name from the UID.
type TypedUID struct {
	Typ   string
	Owner string
	Repo  string
}

func (s TypedUID) Type() string {
	return s.Typ
}

func (s TypedUID) MarshalJSON() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *TypedUID) UnmarshalJSON(data []byte) error {
	t, err := NewTypedUIDFromString(string(data))
	if err != nil {
		return err
	}
	*s = *t.(*TypedUID)
	return nil
}

func (s TypedUID) String() string {
	return fmt.Sprintf("%s:%s:%s", s.Typ, s.Owner, s.Repo)
}

func NewTypedUIDFromString(s string) (types.TypedUID, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid GitHub typed UID: %s", s)
	}
	return &TypedUID{
		Typ:   parts[0],
		Owner: parts[1],
		Repo:  parts[2],
	}, nil
}
