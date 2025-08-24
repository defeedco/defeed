package lib

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TypedUID is a semi-structured ID format for easy resource type extraction.
type TypedUID interface {
	json.Marshaler
	json.Unmarshaler
	Type() string
	String() string
}

type SimpleTypedUID struct {
	Typ         string
	Identifiers []string
}

func (s SimpleTypedUID) Type() string {
	return s.Typ
}

func (s SimpleTypedUID) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *SimpleTypedUID) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	uid, err := NewSimpleTypedUIDFromString(str)
	if err != nil {
		return fmt.Errorf("new source UID from string: %w", err)
	}
	simpleUID, ok := uid.(*SimpleTypedUID)
	if !ok {
		return fmt.Errorf("not a simple typed UID: %s", str)
	}
	*s = *simpleUID
	return nil
}

func (s SimpleTypedUID) String() string {
	ids := make([]string, len(s.Identifiers))
	for i, id := range s.Identifiers {
		// Replace all slashes with colons to avoid conflicts with the URL format,
		// since source UIDs are used as path arguments in URLs.
		ids[i] = strings.ReplaceAll(id, "/", ":")
	}
	return fmt.Sprintf("%s:%s", s.Typ, strings.Join(ids, ":"))
}

func NewSimpleTypedUIDFromString(s string) (TypedUID, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid source UID: %s", s)
	}
	return NewSimpleTypedUID(parts[0], parts[1:]...), nil
}

func NewSimpleTypedUID(sourceType string, identifiers ...string) TypedUID {
	return &SimpleTypedUID{
		Typ:         sourceType,
		Identifiers: identifiers,
	}
}

func Equals(a, b TypedUID) bool {
	return a.String() == b.String()
}
