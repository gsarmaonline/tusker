package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// Slack OpenID Connect endpoints (Sign in with Slack).
var slackEndpoint = oauth2.Endpoint{
	AuthURL:  "https://slack.com/openid/connect/authorize",
	TokenURL: "https://slack.com/api/openid.connect.token",
}

type SlackProvider struct {
	config *oauth2.Config
}

// NewSlackProvider creates a Slack OpenID Connect provider for a specific tenant's credentials.
func NewSlackProvider(clientID, clientSecret, redirectURL string) *SlackProvider {
	return &SlackProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     slackEndpoint,
		},
	}
}

func (s *SlackProvider) AuthURL(state string) string {
	return s.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (s *SlackProvider) Exchange(ctx context.Context, code string) (*Token, error) {
	t, err := s.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("slack token exchange: %w", err)
	}
	return &Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	}, nil
}

func (s *SlackProvider) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	src := s.config.TokenSource(ctx, &oauth2.Token{RefreshToken: refreshToken})
	t, err := src.Token()
	if err != nil {
		return nil, fmt.Errorf("slack token refresh: %w", err)
	}
	return &Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Expiry:       t.Expiry,
	}, nil
}

func (s *SlackProvider) UserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://slack.com/api/openid.connect.userInfo", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("slack userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack userinfo returned %d", resp.StatusCode)
	}

	var body struct {
		OK    bool   `json:"ok"`
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("slack userinfo decode: %w", err)
	}
	if !body.OK || body.Sub == "" {
		return nil, fmt.Errorf("slack userinfo: empty user ID")
	}

	return &UserInfo{ID: body.Sub, Email: body.Email}, nil
}
