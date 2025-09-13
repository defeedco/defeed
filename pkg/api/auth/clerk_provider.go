package auth

import (
	"context"
	"net/http"

	"github.com/clerk/clerk-sdk-go/v2"
	clerkhttp "github.com/clerk/clerk-sdk-go/v2/http"
)

type ClerkAuthProvider struct {
	customClaimsConstructor func(ctx context.Context) any
}

type customSessionClaims struct {
	PrimaryEmail string `json:"primaryEmail"`
}

func NewClerkAuthProviderWithSDK() *ClerkAuthProvider {
	return &ClerkAuthProvider{
		customClaimsConstructor: func(ctx context.Context) any {
			return &customSessionClaims{}
		},
	}
}

func (p *ClerkAuthProvider) Authenticate(next http.Handler) http.Handler {
	authParams := func(params *clerkhttp.AuthorizationParams) error {
		params.VerifyParams.CustomClaimsConstructor = p.customClaimsConstructor
		return nil
	}

	return clerkhttp.WithHeaderAuthorization(authParams)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := clerk.SessionClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		user := &User{
			UserID: claims.Subject,
			Email:  "",
		}

		if customClaims, ok := claims.Custom.(*customSessionClaims); ok {
			user.Email = customClaims.PrimaryEmail
		}

		ctx := context.WithValue(r.Context(), UserContextKey_, user)
		ctx = context.WithValue(ctx, UserContextKey_, user)

		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}
