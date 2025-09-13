package auth

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Config struct {
	// APIKeys is a JSON or comma-separated key=value pairs string containing key-to-userID mapping
	// Example: {"key1":"user1","key2":"user2"} or "key1=user1,key2=user2"
	APIKeys string `env:"AUTH_API_KEYS,default={}"`
}

// ParseAPIKeys parses the JSON string into a map[string]string
func (c *Config) ParseAPIKeys() (map[string]string, error) {
	if c.APIKeys == "" || c.APIKeys == "{}" {
		return make(map[string]string), nil
	}

	var keyMap map[string]string
	if err := json.Unmarshal([]byte(c.APIKeys), &keyMap); err != nil {
		return c.parseKeyValuePairs()
	}

	return keyMap, nil
}

func (c *Config) parseKeyValuePairs() (map[string]string, error) {
	keyMap := make(map[string]string)

	if c.APIKeys == "" {
		return keyMap, nil
	}

	for pair := range strings.SplitSeq(c.APIKeys, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid key-value pair: %s", pair)
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if key == "" || value == "" {
			return nil, fmt.Errorf("empty key or value in pair: %s", pair)
		}

		keyMap[key] = value
	}

	return keyMap, nil
}
