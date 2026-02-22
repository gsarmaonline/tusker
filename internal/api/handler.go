package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gsarma/tusker/internal/crypto"
	"github.com/gsarma/tusker/internal/email"
	"github.com/gsarma/tusker/internal/oauth"
	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/tenant"
)

type Handler struct {
	queries   store.Querier
	tenantSvc *tenant.Service
	enc       *crypto.Encryptor
	executors map[string]Executor
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

	// Auto-refresh if the token is expired or expires within 30 seconds.
	if row.ExpiresAt != nil && time.Until(*row.ExpiresAt) < 30*time.Second {
		row, err = h.refreshAndStore(ctx, t, providerName, userID, row, dataKey)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "token refresh failed"})
			return
		}
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

// refreshAndStore uses the stored refresh token to obtain a new access token,
// encrypts and persists it, and returns the updated row.
func (h *Handler) refreshAndStore(ctx context.Context, t *store.Tenant, providerName, userID string, row store.OauthToken, dataKey []byte) (store.OauthToken, error) {
	if len(row.EncryptedRefreshToken) == 0 {
		return row, fmt.Errorf("no refresh token available")
	}

	refreshToken, err := crypto.DecryptWithDataKey(dataKey, row.EncryptedRefreshToken)
	if err != nil {
		return row, fmt.Errorf("decrypt refresh token: %w", err)
	}

	p, err := h.buildProvider(ctx, t, providerName)
	if err != nil {
		return row, err
	}

	newToken, err := p.Refresh(ctx, string(refreshToken))
	if err != nil {
		return row, fmt.Errorf("provider refresh: %w", err)
	}

	encAccess, err := crypto.EncryptWithDataKey(dataKey, []byte(newToken.AccessToken))
	if err != nil {
		return row, fmt.Errorf("encrypt access token: %w", err)
	}

	// Keep existing refresh token if provider didn't return a new one.
	encRefresh := row.EncryptedRefreshToken
	if newToken.RefreshToken != "" {
		encRefresh, err = crypto.EncryptWithDataKey(dataKey, []byte(newToken.RefreshToken))
		if err != nil {
			return row, fmt.Errorf("encrypt refresh token: %w", err)
		}
	}

	var expiresAt *time.Time
	if !newToken.Expiry.IsZero() {
		expiresAt = &newToken.Expiry
	}

	updated, err := h.queries.UpsertOAuthToken(ctx, store.UpsertOAuthTokenParams{
		TenantID:              t.ID,
		Provider:              providerName,
		UserID:                userID,
		EncryptedAccessToken:  encAccess,
		EncryptedRefreshToken: encRefresh,
		ExpiresAt:             expiresAt,
	})
	if err != nil {
		return row, fmt.Errorf("store refreshed token: %w", err)
	}

	return updated, nil
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

// SetEmailProviderConfig stores a tenant's email provider credentials.
// The request body is the provider-specific JSON config (e.g. SMTP host/port/credentials
// or a SendGrid API key), which is encrypted with the tenant's data key before storage.
func (h *Handler) SetEmailProviderConfig(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")

	configJSON, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}
	if !json.Valid(configJSON) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	dataKey, err := h.tenantSvc.DataKey(t)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	encConfig, err := crypto.EncryptWithDataKey(dataKey, configJSON)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	_, err = h.queries.UpsertEmailProviderConfig(c.Request.Context(), store.UpsertEmailProviderConfigParams{
		TenantID:        t.ID,
		Provider:        providerName,
		EncryptedConfig: encConfig,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save email config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SendEmail queues an email job (async by default) or sends immediately with ?sync=true.
func (h *Handler) SendEmail(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")

	var body struct {
		To      []string `json:"to" binding:"required"`
		From    string   `json:"from" binding:"required"`
		Subject string   `json:"subject" binding:"required"`
		Body    string   `json:"body" binding:"required"`
		HTML    bool     `json:"html"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if c.Query("sync") != "true" {
		payloadJSON, _ := json.Marshal(email.JobPayload{
			Provider: providerName,
			Message: email.Message{
				To:      body.To,
				From:    body.From,
				Subject: body.Subject,
				Body:    body.Body,
				HTML:    body.HTML,
			},
		})
		job, err := h.queries.CreateJob(c.Request.Context(), store.CreateJobParams{
			TenantID: t.ID,
			JobType:  "email.send",
			Payload:  payloadJSON,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue job"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "status": "queued"})
		return
	}

	// Sync path: send immediately.
	p, err := h.buildEmailProvider(c.Request.Context(), t, providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg := email.Message{
		To:      body.To,
		From:    body.From,
		Subject: body.Subject,
		Body:    body.Body,
		HTML:    body.HTML,
	}
	if err := p.Send(c.Request.Context(), msg); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to send email"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "sent"})
}

// GetJob returns the status of a background job scoped to the current tenant.
func (h *Handler) GetJob(c *gin.Context) {
	t := tenant.FromContext(c)
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}
	job, err := h.queries.GetJob(c.Request.Context(), store.GetJobParams{
		ID:       jobID,
		TenantID: t.ID,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job not found"})
		return
	}
	c.JSON(http.StatusOK, job)
}

// buildEmailProvider loads the tenant's email provider credentials and constructs the provider.
func (h *Handler) buildEmailProvider(ctx context.Context, t *store.Tenant, providerName string) (email.Provider, error) {
	cfg, err := h.queries.GetEmailProviderConfig(ctx, store.GetEmailProviderConfigParams{
		TenantID: t.ID,
		Provider: providerName,
	})
	if err != nil {
		return nil, fmt.Errorf("email provider config not found for %s", providerName)
	}

	dataKey, err := h.tenantSvc.DataKey(t)
	if err != nil {
		return nil, fmt.Errorf("encryption error")
	}

	configJSON, err := crypto.DecryptWithDataKey(dataKey, cfg.EncryptedConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt email config")
	}

	switch providerName {
	case "smtp":
		var smtpCfg email.SMTPConfig
		if err := json.Unmarshal(configJSON, &smtpCfg); err != nil {
			return nil, fmt.Errorf("invalid smtp config")
		}
		return email.NewSMTPProvider(smtpCfg), nil
	case "sendgrid":
		var sgCfg email.SendGridConfig
		if err := json.Unmarshal(configJSON, &sgCfg); err != nil {
			return nil, fmt.Errorf("invalid sendgrid config")
		}
		return email.NewSendGridProvider(sgCfg), nil
	default:
		return nil, fmt.Errorf("unsupported email provider: %s", providerName)
	}
}
