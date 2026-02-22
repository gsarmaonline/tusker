package oauth

import (
	"context"
	"time"
)

// Token holds OAuth credentials for a user.
type Token struct {
	AccessToken  string
	RefreshToken string
	Expiry       time.Time
}

// Provider defines the interface each OAuth provider must implement.
type Provider interface {
	// AuthURL returns the URL to redirect the user to for authorization.
	AuthURL(state string) string
	// Exchange converts an authorization code into a Token.
	Exchange(ctx context.Context, code string) (*Token, error)
	// Refresh obtains a new access token using the refresh token.
	Refresh(ctx context.Context, refreshToken string) (*Token, error)
}
