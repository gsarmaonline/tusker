package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/gsarma/tusker/internal/email"
	"github.com/gsarma/tusker/internal/sms"
	"github.com/gsarma/tusker/internal/store"
)

// ExecuteJob dispatches a job to the appropriate handler by type.
// It implements worker.JobExecutor.
func (h *Handler) ExecuteJob(ctx context.Context, tenantID uuid.UUID, jobType string, payload json.RawMessage) error {
	t, err := h.queries.GetTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}
	switch jobType {
	case "email.send":
		return h.executeEmailJob(ctx, &t, payload)
	case "sms.send":
		return h.executeSMSJob(ctx, &t, payload)
	default:
		return fmt.Errorf("unknown job type: %s", jobType)
	}
}

func (h *Handler) executeEmailJob(ctx context.Context, t *store.Tenant, raw json.RawMessage) error {
	var p email.JobPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid email job payload: %w", err)
	}
	provider, err := h.buildEmailProvider(ctx, t, p.Provider)
	if err != nil {
		return err
	}
	return provider.Send(ctx, p.Message)
}

func (h *Handler) executeSMSJob(ctx context.Context, t *store.Tenant, raw json.RawMessage) error {
	var p sms.JobPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid sms job payload: %w", err)
	}
	provider, err := h.buildSMSProvider(ctx, t, p.Provider)
	if err != nil {
		return err
	}
	_, err = provider.Send(ctx, p.From, p.To, p.Body)
	return err
}
