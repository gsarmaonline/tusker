package api

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/gsarma/tusker/internal/code"
	"github.com/gsarma/tusker/internal/email"
	"github.com/gsarma/tusker/internal/sms"
	"github.com/gsarma/tusker/internal/store"
)

// Executor handles async execution of a specific job type.
// Implement this interface and add an instance to registerExecutors to support a new async provider.
type Executor interface {
	JobType() string
	Execute(ctx context.Context, jobID uuid.UUID, t *store.Tenant, payload json.RawMessage) error
}

// registerExecutors builds the handler's job-type dispatch table.
// To add a new async provider, implement Executor and append it here.
func (h *Handler) registerExecutors() {
	execs := []Executor{
		&emailExecutor{h},
		&smsExecutor{h},
		&codeExecutor{h},
	}
	h.executors = make(map[string]Executor, len(execs))
	for _, e := range execs {
		h.executors[e.JobType()] = e
	}
}

// ExecuteJob implements worker.JobExecutor by dispatching to the registered Executor for the job type.
func (h *Handler) ExecuteJob(ctx context.Context, jobID uuid.UUID, tenantID uuid.UUID, jobType string, payload json.RawMessage) error {
	t, err := h.queries.GetTenantByID(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("tenant not found: %w", err)
	}
	exec, ok := h.executors[jobType]
	if !ok {
		return fmt.Errorf("unknown job type: %s", jobType)
	}
	return exec.Execute(ctx, jobID, &t, payload)
}

// emailExecutor handles email.send jobs.
type emailExecutor struct{ h *Handler }

func (e *emailExecutor) JobType() string { return "email.send" }

func (e *emailExecutor) Execute(ctx context.Context, _ uuid.UUID, t *store.Tenant, raw json.RawMessage) error {
	var p email.JobPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid email job payload: %w", err)
	}
	provider, err := e.h.buildEmailProvider(ctx, t, p.Provider)
	if err != nil {
		return err
	}
	return provider.Send(ctx, p.Message)
}

// smsExecutor handles sms.send jobs.
type smsExecutor struct{ h *Handler }

func (e *smsExecutor) JobType() string { return "sms.send" }

func (e *smsExecutor) Execute(ctx context.Context, _ uuid.UUID, t *store.Tenant, raw json.RawMessage) error {
	var p sms.JobPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid sms job payload: %w", err)
	}
	provider, err := e.h.buildSMSProvider(ctx, t, p.Provider)
	if err != nil {
		return err
	}
	_, err = provider.Send(ctx, p.From, p.To, p.Body)
	return err
}

// codeExecutor handles code.execute jobs.
type codeExecutor struct{ h *Handler }

func (e *codeExecutor) JobType() string { return "code.execute" }

func (e *codeExecutor) Execute(ctx context.Context, jobID uuid.UUID, t *store.Tenant, raw json.RawMessage) error {
	var p code.JobPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid code job payload: %w", err)
	}
	provider, err := e.h.buildCodeProvider(ctx, t, p.Provider)
	if err != nil {
		return err
	}
	result, err := provider.Execute(ctx, p.SourceCode, p.LanguageID, p.Stdin)
	if err != nil {
		return err
	}
	_, err = e.h.queries.InsertCodeExecution(ctx, store.InsertCodeExecutionParams{
		JobID:         jobID,
		TenantID:      t.ID,
		Stdout:        result.Stdout,
		Stderr:        result.Stderr,
		CompileOutput: result.CompileOutput,
		Status:        result.Status,
		ExecTime:      result.Time,
		Memory:        int32(result.Memory),
	})
	return err
}
