package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gsarma/tusker/internal/crypto"
	"github.com/gsarma/tusker/internal/oauth"
	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/tenant"
)

type Handler struct {
	queries   *store.Queries
	tenantSvc *tenant.Service
	enc       *crypto.Encryptor
}

// CreateTenant provisions a new tenant and returns the API key (shown once).
func (h *Handler) CreateTenant(c *gin.Context) {
	apiKey, tenantID, err := h.tenantSvc.Create(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create tenant"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"tenant_id": tenantID,
		"api_key":   apiKey,
		"note":      "Store this API key — it will not be shown again.",
	})
}

// SetProviderConfig stores a tenant's OAuth client credentials for a provider.
func (h *Handler) SetProviderConfig(c *gin.Context) {
	t := tenant.FromContext(c)
	provider := c.Param("provider")

	var body struct {
		ClientID     string `json:"client_id" binding:"required"`
		ClientSecret string `json:"client_secret" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dataKey, err := h.tenantSvc.DataKey(t)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	encSecret, err := crypto.EncryptWithDataKey(dataKey, []byte(body.ClientSecret))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	_, err = h.queries.UpsertProviderConfig(c.Request.Context(), store.UpsertProviderConfigParams{
		TenantID:              t.ID,
		Provider:              provider,
		ClientID:              body.ClientID,
		EncryptedClientSecret: encSecret,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Authorize initiates the OAuth flow by redirecting to the provider.
func (h *Handler) Authorize(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")
	redirectURI := c.Query("redirect_uri")
	if redirectURI == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "redirect_uri is required"})
		return
	}

	p, err := h.buildProvider(c.Request.Context(), t, providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	state, err := oauth.EncodeState(t.ID, redirectURI)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate state"})
		return
	}

	c.Redirect(http.StatusFound, p.AuthURL(state))
}

// Callback handles the provider redirect after user authorization.
func (h *Handler) Callback(c *gin.Context) {
	providerName := c.Param("provider")
	code := c.Query("code")
	stateParam := c.Query("state")

	if code == "" || stateParam == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing code or state"})
		return
	}

	state, err := oauth.DecodeState(stateParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid state"})
		return
	}

	ctx := c.Request.Context()

	t, err := h.queries.GetTenantByID(ctx, state.TenantID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown tenant"})
		return
	}

	p, err := h.buildProvider(ctx, &t, providerName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	token, err := p.Exchange(ctx, code)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "token exchange failed"})
		return
	}

	userInfo, err := p.UserInfo(ctx, token.AccessToken)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to fetch user info"})
		return
	}

	dataKey, err := h.tenantSvc.DataKey(&t)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	encAccess, err := crypto.EncryptWithDataKey(dataKey, []byte(token.AccessToken))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	var encRefresh []byte
	if token.RefreshToken != "" {
		encRefresh, err = crypto.EncryptWithDataKey(dataKey, []byte(token.RefreshToken))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
			return
		}
	}

	var expiresAt *time.Time
	if !token.Expiry.IsZero() {
		expiresAt = &token.Expiry
	}

	_, err = h.queries.UpsertOAuthToken(ctx, store.UpsertOAuthTokenParams{
		TenantID:              t.ID,
		Provider:              providerName,
		UserID:                userInfo.ID,
		EncryptedAccessToken:  encAccess,
		EncryptedRefreshToken: encRefresh,
		ExpiresAt:             expiresAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store token"})
		return
	}

	redirectURL := state.RedirectURI + "?user_id=" + userInfo.ID
	c.Redirect(http.StatusFound, redirectURL)
}

// GetToken returns the decrypted access token for a provider/user.
func (h *Handler) GetToken(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")
	userID := c.DefaultQuery("user_id", "default")

	ctx := c.Request.Context()

	row, err := h.queries.GetOAuthToken(ctx, store.GetOAuthTokenParams{
		TenantID: t.ID,
		Provider: providerName,
		UserID:   userID,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "token not found"})
		return
	}

	dataKey, err := h.tenantSvc.DataKey(t)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	accessToken, err := crypto.DecryptWithDataKey(dataKey, row.EncryptedAccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "decryption error"})
		return
	}

	resp := gin.H{
		"access_token": string(accessToken),
		"provider":     providerName,
		"user_id":      userID,
	}
	if row.ExpiresAt != nil {
		resp["expires_at"] = row.ExpiresAt
	}

	c.JSON(http.StatusOK, resp)
}

// DeleteToken revokes a stored OAuth token.
func (h *Handler) DeleteToken(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")
	userID := c.DefaultQuery("user_id", "default")

	err := h.queries.DeleteOAuthToken(c.Request.Context(), store.DeleteOAuthTokenParams{
		TenantID: t.ID,
		Provider: providerName,
		UserID:   userID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// buildProvider loads tenant credentials and constructs the named provider.
func (h *Handler) buildProvider(ctx context.Context, t *store.Tenant, providerName string) (oauth.Provider, error) {
	cfg, err := h.queries.GetProviderConfig(ctx, store.GetProviderConfigParams{
		TenantID: t.ID,
		Provider: providerName,
	})
	if err != nil {
		return nil, fmt.Errorf("provider config not found for %s", providerName)
	}

	dataKey, err := h.tenantSvc.DataKey(t)
	if err != nil {
		return nil, fmt.Errorf("encryption error")
	}

	clientSecret, err := crypto.DecryptWithDataKey(dataKey, cfg.EncryptedClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt client secret")
	}

	baseURL := os.Getenv("TUSKER_BASE_URL")
	callbackURL := fmt.Sprintf("%s/oauth/%s/callback", baseURL, providerName)

	switch providerName {
	case "google":
		return oauth.NewGoogleProvider(cfg.ClientID, string(clientSecret), callbackURL), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}
