package github

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/defeedco/defeed/pkg/sources/activities/types"
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
	return json.Marshal(s.String())
}

func (s *TypedUID) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	t, err := NewTypedUIDFromString(str)
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
