package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/gsarma/tusker/internal/code"
	"github.com/gsarma/tusker/internal/crypto"
	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/tenant"
)

// SetCodeProviderConfig stores a tenant's code execution provider config (encrypted).
// For Judge0 the body is: {"url": "http://...", "auth_token": "optional"}
// If not set, the server falls back to the JUDGE0_URL environment variable.
func (h *Handler) SetCodeProviderConfig(c *gin.Context) {
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

	_, err = h.queries.UpsertCodeProviderConfig(c.Request.Context(), store.UpsertCodeProviderConfigParams{
		TenantID:        t.ID,
		Provider:        providerName,
		EncryptedConfig: encConfig,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ExecuteCode queues a code execution job (async by default) or runs immediately with ?sync=true.
//
// Request body:
//
//	{
//	  "source_code": "print('hello')",
//	  "language_id": 71,         // Judge0 language ID (71 = Python 3)
//	  "stdin":       "optional"
//	}
//
// Async (default): returns 202 {"job_id": "...", "status": "queued"}.
// Sync (?sync=true): returns 200 with the execution result directly.
// After async completion, retrieve results via GET /code/executions/:job_id.
func (h *Handler) ExecuteCode(c *gin.Context) {
	t := tenant.FromContext(c)
	providerName := c.Param("provider")

	var body struct {
		SourceCode string `json:"source_code" binding:"required"`
		LanguageID int    `json:"language_id" binding:"required"`
		Stdin      string `json:"stdin"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if c.Query("sync") != "true" {
		payloadJSON, _ := json.Marshal(code.JobPayload{
			Provider:   providerName,
			SourceCode: body.SourceCode,
			LanguageID: body.LanguageID,
			Stdin:      body.Stdin,
		})
		job, err := h.queries.CreateJob(c.Request.Context(), store.CreateJobParams{
			TenantID: t.ID,
			JobType:  "code.execute",
			Payload:  payloadJSON,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue job"})
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"job_id": job.ID, "status": "queued"})
		return
	}

	// Sync path: execute immediately and return the result.
	p, err := h.buildCodeProvider(c.Request.Context(), t, providerName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := p.Execute(c.Request.Context(), body.SourceCode, body.LanguageID, body.Stdin)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetCodeExecution returns the stored output of a completed code.execute job.
// Call this after GET /jobs/:id reports status "completed".
func (h *Handler) GetCodeExecution(c *gin.Context) {
	t := tenant.FromContext(c)
	jobID, err := uuid.Parse(c.Param("job_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid job id"})
		return
	}

	exec, err := h.queries.GetCodeExecution(c.Request.Context(), store.GetCodeExecutionParams{
		JobID:    jobID,
		TenantID: t.ID,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "execution result not found"})
		return
	}

	c.JSON(http.StatusOK, exec)
}

// buildCodeProvider loads tenant credentials (if any) and constructs the named code provider.
// Falls back to the JUDGE0_URL environment variable when no tenant config is stored.
func (h *Handler) buildCodeProvider(ctx context.Context, t *store.Tenant, providerName string) (code.Provider, error) {
	switch providerName {
	case "judge0":
		cfg := code.Judge0Config{
			URL: os.Getenv("JUDGE0_URL"),
		}
		if cfg.URL == "" {
			cfg.URL = "http://judge0-server:2358"
		}

		// Allow tenants to override the URL and set an auth token.
		tenantCfg, err := h.queries.GetCodeProviderConfig(ctx, store.GetCodeProviderConfigParams{
			TenantID: t.ID,
			Provider: providerName,
		})
		if err == nil {
			dataKey, dkErr := h.tenantSvc.DataKey(t)
			if dkErr == nil {
				if configJSON, decErr := crypto.DecryptWithDataKey(dataKey, tenantCfg.EncryptedConfig); decErr == nil {
					var override code.Judge0Config
					if json.Unmarshal(configJSON, &override) == nil {
						if override.URL != "" {
							cfg.URL = override.URL
						}
						cfg.AuthToken = override.AuthToken
					}
				}
			}
		}

		return code.NewJudge0Provider(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported code provider: %s", providerName)
	}
}
