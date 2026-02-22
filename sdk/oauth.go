package tusker

import (
	"context"
	"fmt"
	"net/http"
)

// OAuthService provides OAuth configuration and token operations.
type OAuthService struct {
	c *Client
}

// SetConfig stores OAuth client credentials for the given provider.
// Supported providers: "google"
func (s *OAuthService) SetConfig(ctx context.Context, provider string, req SetOAuthConfigRequest) error {
	path := fmt.Sprintf("/oauth/%s/config", provider)
	_, err := doRequest[StatusResponse](ctx, s.c, http.MethodPost, path, req, http.StatusOK)
	return err
}

// GetAuthorizeURL returns the OAuth authorization URL that your user should
// be redirected to. redirectURI is where Tusker will redirect after the user
// grants access; Tusker will append ?user_id=... to this URI on success.
func (s *OAuthService) GetAuthorizeURL(provider, redirectURI string) string {
	u := fmt.Sprintf("%s/oauth/%s/authorize?redirect_uri=%s",
		s.c.baseURL, provider, redirectURI)
	return u
}

// GetToken retrieves the stored (and auto-refreshed) access token for a user.
// userID defaults to "default" if empty.
func (s *OAuthService) GetToken(ctx context.Context, provider, userID string) (*GetOAuthTokenResponse, error) {
	path := fmt.Sprintf("/oauth/%s/token", provider)
	query := map[string]string{}
	if userID != "" {
		query["user_id"] = userID
	}
	return doRequestWithQuery[GetOAuthTokenResponse](ctx, s.c, http.MethodGet, path, query, nil, http.StatusOK)
}

// DeleteToken revokes and removes the stored token for a user.
// userID defaults to "default" if empty.
func (s *OAuthService) DeleteToken(ctx context.Context, provider, userID string) error {
	path := fmt.Sprintf("/oauth/%s/token", provider)
	query := map[string]string{}
	if userID != "" {
		query["user_id"] = userID
	}
	_, err := doRequestWithQuery[StatusResponse](ctx, s.c, http.MethodDelete, path, query, nil, http.StatusOK)
	return err
}
