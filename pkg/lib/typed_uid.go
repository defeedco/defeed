package lib

import (
	"encoding/json"
	"fmt"
	"github.com/glanceapp/glance/pkg/sources/activities/types"
	"strings"
)

// TypedUID is the default implementation of TypedUID interface
type TypedUID struct {
	Typ         string
	Identifiers []string
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
	uid, err := NewTypedUIDFromString(str)
	if err != nil {
		return fmt.Errorf("new source UID from string: %w", err)
	}
	simpleUID, ok := uid.(*TypedUID)
	if !ok {
		return fmt.Errorf("not a simple typed UID: %s", str)
	}
	*s = *simpleUID
	return nil
}

func (s TypedUID) String() string {
	ids := make([]string, len(s.Identifiers))
	for i, id := range s.Identifiers {
		// Replace all slashes with colons to avoid conflicts with the URL format,
		// since source UIDs are used as path arguments in URLs.
		ids[i] = strings.ReplaceAll(id, "/", ":")
	}
	return fmt.Sprintf("%s:%s", s.Typ, strings.Join(ids, ":"))
}

func NewTypedUIDFromString(s string) (types.TypedUID, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid source UID: %s", s)
	}
	return NewTypedUID(parts[0], parts[1:]...), nil
}

func NewTypedUID(sourceType string, identifiers ...string) types.TypedUID {
	return &TypedUID{
		Typ:         sourceType,
		Identifiers: identifiers,
	}
}

func Equals(a, b types.TypedUID) bool {
	return a.String() == b.String()
}
