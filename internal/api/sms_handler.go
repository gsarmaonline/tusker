package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/gsarma/tusker/internal/crypto"
	"github.com/gsarma/tusker/internal/sms"
	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/tenant"
)

// SetSMSProviderConfig stores a tenant's SMS provider credentials.
// For Twilio: account_sid maps to client_id and auth_token to encrypted_client_secret
// in the shared oauth_provider_configs table.
func (h *Handler) SetSMSProviderConfig(c *gin.Context) {
	t := tenant.FromContext(c)
	provider := c.Param("provider")

	var body struct {
		AccountSID string `json:"account_sid" binding:"required"`
		AuthToken  string `json:"auth_token" binding:"required"`
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

	encToken, err := crypto.EncryptWithDataKey(dataKey, []byte(body.AuthToken))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "encryption error"})
		return
	}

	_, err = h.queries.UpsertProviderConfig(c.Request.Context(), store.UpsertProviderConfigParams{
		TenantID:              t.ID,
		Provider:              provider,
		ClientID:              body.AccountSID,
		EncryptedClientSecret: encToken,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// SendSMS sends an SMS message via the configured provider.
func (h *Handler) SendSMS(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")

	var body struct {
		From string `json:"from" binding:"required"`
		To   string `json:"to" binding:"required"`
		Body string `json:"body" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	p, err := h.buildSMSProvider(ctx, t, providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := p.Send(ctx, body.From, body.To, body.Body)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("send failed: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message_sid": msg.SID,
		"status":      msg.Status,
		"from":        body.From,
		"to":          body.To,
	})
}

// buildSMSProvider loads tenant credentials and constructs the named SMS provider.
func (h *Handler) buildSMSProvider(ctx context.Context, t *store.Tenant, providerName string) (sms.Provider, error) {
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

	authToken, err := crypto.DecryptWithDataKey(dataKey, cfg.EncryptedClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt auth token")
	}

	switch providerName {
	case "twilio":
		return sms.NewTwilioProvider(cfg.ClientID, string(authToken)), nil
	default:
		return nil, fmt.Errorf("unsupported SMS provider: %s", providerName)
	}
}
