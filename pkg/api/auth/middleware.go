package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type RouteAuthMiddleware struct {
	routes      RouteAuthConfig
	defaultAuth *AuthConfig
}

type UserContextKey string

const UserContextKey_ UserContextKey = "user"

type Provider interface {
	// Authenticate is called to authenticate the request.
	// Provider must update the context with the user and call next.ServeHTTP or return a http error.
	Authenticate(next http.Handler, required bool) http.Handler
}

type User struct {
	UserID string
	// Email can be empty (e.g. when using key provider)
	Email string
}

type AuthConfig struct {
	Provider Provider
	Required bool
}

type RouteAuthConfig map[string]AuthConfig

func UserFromContext(ctx context.Context) (User, error) {
	user, ok := ctx.Value(UserContextKey_).(User)
	if !ok {
		return User{}, errors.New("user not found in context")
	}
	return user, nil
}

func NewRouteAuthMiddleware(defaultAuth *AuthConfig) *RouteAuthMiddleware {
	return &RouteAuthMiddleware{
		routes:      make(RouteAuthConfig),
		defaultAuth: defaultAuth,
	}
}

func (m *RouteAuthMiddleware) SetRouteAuth(pattern string, config AuthConfig) *RouteAuthMiddleware {
	m.routes[pattern] = config
	return m
}

func (m *RouteAuthMiddleware) SetRouteAuthProvider(pattern string, provider Provider, required bool) *RouteAuthMiddleware {
	m.routes[pattern] = AuthConfig{
		Provider: provider,
		Required: required,
	}
	return m
}

func (m *RouteAuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for OPTIONS (CORS preflight) requests
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Skip auth for docs endpoints
		if strings.HasPrefix(r.URL.Path, "/docs") {
			next.ServeHTTP(w, r)
			return
		}

		authConfig := m.getAuthConfigForRoute(r.URL.Path, r.Method)

		if authConfig == nil || authConfig.Provider == nil {
			// No auth configured, continue without authentication
			next.ServeHTTP(w, r)
			return
		}

		// Apply the auth middleware
		authMiddleware := authConfig.Provider.Authenticate(next, authConfig.Required)
		authMiddleware.ServeHTTP(w, r)
	})
}

func (m *RouteAuthMiddleware) getAuthConfigForRoute(path, method string) *AuthConfig {
	// Try to find exact match first
	routeKey := method + " " + path
	if config, exists := m.routes[routeKey]; exists {
		return &config
	}

	// Try to find pattern matches
	for pattern, config := range m.routes {
		if m.matchesPattern(pattern, routeKey) {
			return &config
		}
	}

	// Return default auth config
	return m.defaultAuth
}

func (m *RouteAuthMiddleware) matchesPattern(pattern, route string) bool {
	// Simple pattern matching - can be enhanced for more complex patterns
	if strings.Contains(pattern, "{") {
		// Handle path parameters like /feeds/{uid}
		patternParts := strings.Split(pattern, "/")
		routeParts := strings.Split(route, "/")

		if len(patternParts) != len(routeParts) {
			return false
		}

		for i, part := range patternParts {
			if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
				// This is a path parameter, skip comparison
				continue
			}
			if part != routeParts[i] {
				return false
			}
		}
		return true
	}

	return pattern == route
}
