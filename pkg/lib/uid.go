package lib

import (
	"encoding/json"
	"fmt"
	"strings"
)

// TypedUID is a structured ID format for easy resource type extraction.
type TypedUID struct {
	Type        string
	Identifiers []string
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
	*s = uid
	return nil
}

func (s TypedUID) String() string {
	ids := make([]string, len(s.Identifiers))
	for i, id := range s.Identifiers {
		// Replace all slashes with colons to avoid conflicts with the URL format,
		// since source UIDs are used as path arguments in URLs.
		ids[i] = strings.ReplaceAll(id, "/", ":")
	}
	return fmt.Sprintf("%s:%s", s.Type, strings.Join(ids, ":"))
}

func NewTypedUIDFromString(s string) (TypedUID, error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return TypedUID{}, fmt.Errorf("invalid source UID: %s", s)
	}
	return NewTypedUID(parts[0], parts[1:]...), nil
}

func NewTypedUID(sourceType string, identifiers ...string) TypedUID {
	return TypedUID{
		Type:        sourceType,
		Identifiers: identifiers,
	}
}
