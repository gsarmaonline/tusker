package worker_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gsarma/tusker/internal/store"
	"github.com/gsarma/tusker/internal/worker"
)

// stubQuerier implements store.Querier for worker tests.
// Only ClaimNextJob and UpdateJobStatus are exercised; all others return zero values.
type stubQuerier struct {
	claimNextJobFn    func(ctx context.Context) (store.Job, error)
	updateJobStatusFn func(ctx context.Context, arg store.UpdateJobStatusParams) (store.Job, error)
}

func (s *stubQuerier) ClaimNextJob(ctx context.Context) (store.Job, error) {
	if s.claimNextJobFn != nil {
		return s.claimNextJobFn(ctx)
	}
	return store.Job{}, pgx.ErrNoRows
}
func (s *stubQuerier) UpdateJobStatus(ctx context.Context, arg store.UpdateJobStatusParams) (store.Job, error) {
	if s.updateJobStatusFn != nil {
		return s.updateJobStatusFn(ctx, arg)
	}
	return store.Job{}, nil
}
func (s *stubQuerier) CreateJob(ctx context.Context, arg store.CreateJobParams) (store.Job, error) {
	return store.Job{}, nil
}
func (s *stubQuerier) CreateTenant(ctx context.Context, arg store.CreateTenantParams) (store.Tenant, error) {
	return store.Tenant{}, nil
}
func (s *stubQuerier) DeleteOAuthToken(ctx context.Context, arg store.DeleteOAuthTokenParams) error {
	return nil
}
func (s *stubQuerier) GetEmailProviderConfig(ctx context.Context, arg store.GetEmailProviderConfigParams) (store.EmailProviderConfig, error) {
	return store.EmailProviderConfig{}, nil
}
func (s *stubQuerier) GetJob(ctx context.Context, arg store.GetJobParams) (store.Job, error) {
	return store.Job{}, nil
}
func (s *stubQuerier) GetOAuthToken(ctx context.Context, arg store.GetOAuthTokenParams) (store.OauthToken, error) {
	return store.OauthToken{}, nil
}
func (s *stubQuerier) GetProviderConfig(ctx context.Context, arg store.GetProviderConfigParams) (store.OauthProviderConfig, error) {
	return store.OauthProviderConfig{}, nil
}
func (s *stubQuerier) GetTenantByAPIKeyHash(ctx context.Context, apiKeyHash string) (store.Tenant, error) {
	return store.Tenant{}, nil
}
func (s *stubQuerier) GetTenantByID(ctx context.Context, id uuid.UUID) (store.Tenant, error) {
	return store.Tenant{}, nil
}
func (s *stubQuerier) UpsertEmailProviderConfig(ctx context.Context, arg store.UpsertEmailProviderConfigParams) (store.EmailProviderConfig, error) {
	return store.EmailProviderConfig{}, nil
}
func (s *stubQuerier) UpsertOAuthToken(ctx context.Context, arg store.UpsertOAuthTokenParams) (store.OauthToken, error) {
	return store.OauthToken{}, nil
}
func (s *stubQuerier) UpsertProviderConfig(ctx context.Context, arg store.UpsertProviderConfigParams) (store.OauthProviderConfig, error) {
	return store.OauthProviderConfig{}, nil
}
func (s *stubQuerier) DeleteEmailTemplate(ctx context.Context, arg store.DeleteEmailTemplateParams) error {
	return nil
}
func (s *stubQuerier) GetEmailTemplate(ctx context.Context, arg store.GetEmailTemplateParams) (store.EmailTemplate, error) {
	return store.EmailTemplate{}, nil
}
func (s *stubQuerier) ListEmailTemplates(ctx context.Context, tenantID uuid.UUID) ([]store.EmailTemplate, error) {
	return nil, nil
}
func (s *stubQuerier) UpsertEmailTemplate(ctx context.Context, arg store.UpsertEmailTemplateParams) (store.EmailTemplate, error) {
	return store.EmailTemplate{}, nil
}
func (s *stubQuerier) GetCodeProviderConfig(ctx context.Context, arg store.GetCodeProviderConfigParams) (store.CodeProviderConfig, error) {
	return store.CodeProviderConfig{}, nil
}
func (s *stubQuerier) UpsertCodeProviderConfig(ctx context.Context, arg store.UpsertCodeProviderConfigParams) (store.CodeProviderConfig, error) {
	return store.CodeProviderConfig{}, nil
}
func (s *stubQuerier) InsertCodeExecution(ctx context.Context, arg store.InsertCodeExecutionParams) (store.CodeExecution, error) {
	return store.CodeExecution{}, nil
}
func (s *stubQuerier) GetCodeExecution(ctx context.Context, arg store.GetCodeExecutionParams) (store.CodeExecution, error) {
	return store.CodeExecution{}, nil
}

// stubExecutor implements worker.JobExecutor for tests.
type stubExecutor struct {
	executeJobFn func(ctx context.Context, jobID uuid.UUID, tenantID uuid.UUID, jobType string, payload json.RawMessage) error
}

func (s *stubExecutor) ExecuteJob(ctx context.Context, jobID uuid.UUID, tenantID uuid.UUID, jobType string, payload json.RawMessage) error {
	if s.executeJobFn != nil {
		return s.executeJobFn(ctx, jobID, tenantID, jobType, payload)
	}
	return nil
}

// runWorkerUntilDone starts a single-goroutine worker and waits for done to be closed or the test to time out.
func runWorkerUntilDone(t *testing.T, q store.Querier, exec worker.JobExecutor, done <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	w := worker.New(q, exec, 1)
	go w.Start(ctx)
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("timed out waiting for worker to process job")
	}
}

func makeJob(attempt, maxAttempts int32) store.Job {
	return store.Job{
		ID:          uuid.New(),
		TenantID:    uuid.New(),
		JobType:     "email.send",
		Payload:     []byte(`{}`),
		Status:      "running",
		Attempt:     attempt,
		MaxAttempts: maxAttempts,
		RunAt:       time.Now(),
	}
}

func TestWorker_NoJobs(t *testing.T) {
	// When no jobs are pending the worker should not call UpdateJobStatus.
	updateCalled := false
	q := &stubQuerier{
		updateJobStatusFn: func(_ context.Context, _ store.UpdateJobStatusParams) (store.Job, error) {
			updateCalled = true
			return store.Job{}, nil
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	w := worker.New(q, &stubExecutor{}, 1)
	w.Start(ctx) // blocks until timeout
	if updateCalled {
		t.Error("UpdateJobStatus should not be called when there are no jobs")
	}
}

func TestWorker_JobSucceeds(t *testing.T) {
	job := makeJob(1, 3)
	var captured store.UpdateJobStatusParams
	done := make(chan struct{})

	var claimCount int
	q := &stubQuerier{
		claimNextJobFn: func(_ context.Context) (store.Job, error) {
			claimCount++
			if claimCount == 1 {
				return job, nil
			}
			return store.Job{}, pgx.ErrNoRows
		},
		updateJobStatusFn: func(_ context.Context, arg store.UpdateJobStatusParams) (store.Job, error) {
			captured = arg
			close(done)
			return store.Job{}, nil
		},
	}
	runWorkerUntilDone(t, q, &stubExecutor{}, done)

	if captured.Status != "completed" {
		t.Errorf("expected status=completed, got %s", captured.Status)
	}
	if captured.CompletedAt == nil {
		t.Error("expected CompletedAt to be set on success")
	}
	if captured.Error.Valid {
		t.Error("expected Error to be null on success")
	}
}

func TestWorker_JobFailsWithRetry(t *testing.T) {
	// attempt=1, max_attempts=3 → should reset to pending with a future run_at.
	job := makeJob(1, 3)
	execErr := errors.New("provider timeout")
	var captured store.UpdateJobStatusParams
	done := make(chan struct{})

	var claimCount int
	q := &stubQuerier{
		claimNextJobFn: func(_ context.Context) (store.Job, error) {
			claimCount++
			if claimCount == 1 {
				return job, nil
			}
			return store.Job{}, pgx.ErrNoRows
		},
		updateJobStatusFn: func(_ context.Context, arg store.UpdateJobStatusParams) (store.Job, error) {
			captured = arg
			close(done)
			return store.Job{}, nil
		},
	}
	exec := &stubExecutor{
		executeJobFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ json.RawMessage) error {
			return execErr
		},
	}
	runWorkerUntilDone(t, q, exec, done)

	if captured.Status != "pending" {
		t.Errorf("expected status=pending for retry, got %s", captured.Status)
	}
	if !captured.Error.Valid || captured.Error.String != execErr.Error() {
		t.Errorf("expected error=%q, got %+v", execErr.Error(), captured.Error)
	}
	if captured.RunAt.Before(time.Now()) {
		t.Error("expected run_at to be in the future for retry backoff")
	}
	if captured.CompletedAt != nil {
		t.Error("expected CompletedAt to be nil on retry")
	}
}

func TestWorker_JobExhaustsRetries(t *testing.T) {
	// attempt=3, max_attempts=3 → should mark as failed, not retry.
	job := makeJob(3, 3)
	execErr := errors.New("permanent failure")
	var captured store.UpdateJobStatusParams
	done := make(chan struct{})

	var claimCount int
	q := &stubQuerier{
		claimNextJobFn: func(_ context.Context) (store.Job, error) {
			claimCount++
			if claimCount == 1 {
				return job, nil
			}
			return store.Job{}, pgx.ErrNoRows
		},
		updateJobStatusFn: func(_ context.Context, arg store.UpdateJobStatusParams) (store.Job, error) {
			captured = arg
			close(done)
			return store.Job{}, nil
		},
	}
	exec := &stubExecutor{
		executeJobFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ json.RawMessage) error {
			return execErr
		},
	}
	runWorkerUntilDone(t, q, exec, done)

	if captured.Status != "failed" {
		t.Errorf("expected status=failed after exhausting retries, got %s", captured.Status)
	}
	if !captured.Error.Valid || captured.Error.String != execErr.Error() {
		t.Errorf("expected error=%q, got %+v", execErr.Error(), captured.Error)
	}
}

func TestWorker_BackoffGrowsWithAttempt(t *testing.T) {
	// Each successive retry should schedule run_at further into the future.
	cases := []struct {
		attempt     int32
		minBackoff  time.Duration
	}{
		{1, 20*time.Second - time.Second},  // 2^1 * 10s = 20s
		{2, 40*time.Second - time.Second},  // 2^2 * 10s = 40s
		{3, 80*time.Second - time.Second},  // 2^3 * 10s = 80s (but max_attempts=4)
	}

	for _, tc := range cases {
		tc := tc
		t.Run("attempt"+string(rune('0'+tc.attempt)), func(t *testing.T) {
			job := makeJob(tc.attempt, 10) // max_attempts=10 so always retries
			var captured store.UpdateJobStatusParams
			done := make(chan struct{})
			var claimCount int
			q := &stubQuerier{
				claimNextJobFn: func(_ context.Context) (store.Job, error) {
					claimCount++
					if claimCount == 1 {
						return job, nil
					}
					return store.Job{}, pgx.ErrNoRows
				},
				updateJobStatusFn: func(_ context.Context, arg store.UpdateJobStatusParams) (store.Job, error) {
					captured = arg
					close(done)
					return store.Job{}, nil
				},
			}
			exec := &stubExecutor{
				executeJobFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID, _ string, _ json.RawMessage) error {
					return errors.New("fail")
				},
			}
			runWorkerUntilDone(t, q, exec, done)

			minRunAt := time.Now().Add(tc.minBackoff)
			if captured.RunAt.Before(minRunAt) {
				t.Errorf("attempt %d: expected run_at >= %v, got %v", tc.attempt, minRunAt, captured.RunAt)
			}

			// Verify pgtype.Text is set correctly
			if captured.Error.String != "fail" {
				t.Errorf("expected error=fail, got %q", captured.Error.String)
			}
		})
	}
}

// Compile-time check: stubQuerier satisfies store.Querier.
var _ store.Querier = (*stubQuerier)(nil)

// Compile-time check: stubExecutor satisfies worker.JobExecutor.
var _ worker.JobExecutor = (*stubExecutor)(nil)

// Confirm pgtype.Text works as expected in tests.
func TestPgtypeText(t *testing.T) {
	valid := pgtype.Text{String: "hello", Valid: true}
	if !valid.Valid || valid.String != "hello" {
		t.Error("pgtype.Text with value should be valid")
	}
	null := pgtype.Text{Valid: false}
	if null.Valid {
		t.Error("pgtype.Text zero value should be null")
	}
}
