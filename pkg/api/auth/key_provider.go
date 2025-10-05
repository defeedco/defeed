package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type KeyAuthProvider struct {
	keyToUserID map[string]string
}

func NewKeyAuthProvider(keyToUserID map[string]string) *KeyAuthProvider {
	return &KeyAuthProvider{
		keyToUserID: keyToUserID,
	}
}

func (p *KeyAuthProvider) Authenticate(next http.Handler, required bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		userID, err := p.resolveUserID(authHeader)
		if err != nil && required {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		user := User{
			UserID: userID,
			Email:  "",
		}

		ctx := context.WithValue(r.Context(), UserContextKey_, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (p *KeyAuthProvider) resolveUserID(authHeader string) (string, error) {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return "", fmt.Errorf("invalid authorization header")
	}

	authToken := strings.TrimPrefix(authHeader, "Bearer ")
	if authToken == "" {
		return "", fmt.Errorf("invalid auth token format")
	}

	userID, ok := p.keyToUserID[authToken]
	if !ok {
		return "", fmt.Errorf("unauthorized")
	}

	return userID, nil
}
