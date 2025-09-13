package auth

import (
	"context"
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

func (p *KeyAuthProvider) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "invalid authorization header", http.StatusBadRequest)
			return
		}

		authToken := strings.TrimPrefix(authHeader, "Bearer ")
		if authToken == "" {
			http.Error(w, "invalid auth token format", http.StatusBadRequest)
			return
		}

		userID, ok := p.keyToUserID[authToken]
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
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
