package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/gsarma/tusker/internal/store"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// stubQuerier implements store.Querier for api handler tests.
// Only CreateJob and GetJob are wired up; all others return zero values.
type stubQuerier struct {
	createJobFn func(ctx context.Context, arg store.CreateJobParams) (store.Job, error)
	getJobFn    func(ctx context.Context, arg store.GetJobParams) (store.Job, error)
}

func (s *stubQuerier) CreateJob(ctx context.Context, arg store.CreateJobParams) (store.Job, error) {
	if s.createJobFn != nil {
		return s.createJobFn(ctx, arg)
	}
	return store.Job{}, nil
}
func (s *stubQuerier) GetJob(ctx context.Context, arg store.GetJobParams) (store.Job, error) {
	if s.getJobFn != nil {
		return s.getJobFn(ctx, arg)
	}
	return store.Job{}, pgx.ErrNoRows
}
func (s *stubQuerier) ClaimNextJob(ctx context.Context) (store.Job, error) {
	return store.Job{}, nil
}
func (s *stubQuerier) UpdateJobStatus(ctx context.Context, arg store.UpdateJobStatusParams) (store.Job, error) {
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

// Compile-time interface check.
var _ store.Querier = (*stubQuerier)(nil)

// ginCtx builds a Gin test context with an authenticated tenant already set.
func ginCtx(method, path string, body []byte, tenantID uuid.UUID, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Params = params
	c.Set("tenant", &store.Tenant{ID: tenantID})
	return c, w
}

// --- SendEmail tests ---

func TestSendEmail_Async_CreatesJobAndReturns202(t *testing.T) {
	tenantID := uuid.New()
	createdJob := store.Job{ID: uuid.New(), Status: "pending"}

	var gotParams store.CreateJobParams
	q := &stubQuerier{
		createJobFn: func(_ context.Context, arg store.CreateJobParams) (store.Job, error) {
			gotParams = arg
			return createdJob, nil
		},
	}
	h := &Handler{queries: q}

	body, _ := json.Marshal(map[string]interface{}{
		"to": []string{"bob@example.com"}, "from": "alice@example.com",
		"subject": "Hi", "body": "Hello",
	})
	c, w := ginCtx("POST", "/email/smtp/send", body, tenantID, gin.Params{{Key: "provider", Value: "smtp"}})
	h.SendEmail(c)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["job_id"] == nil {
		t.Error("expected job_id in response")
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status=queued, got %v", resp["status"])
	}
	if gotParams.JobType != "email.send" {
		t.Errorf("expected job_type=email.send, got %s", gotParams.JobType)
	}
	if gotParams.TenantID != tenantID {
		t.Error("job should be scoped to the request tenant")
	}
}

func TestSendEmail_Async_StoreError_Returns500(t *testing.T) {
	q := &stubQuerier{
		createJobFn: func(_ context.Context, _ store.CreateJobParams) (store.Job, error) {
			return store.Job{}, pgx.ErrTxClosed
		},
	}
	h := &Handler{queries: q}

	body, _ := json.Marshal(map[string]interface{}{
		"to": []string{"a@b.com"}, "from": "x@y.com", "subject": "s", "body": "b",
	})
	c, w := ginCtx("POST", "/email/smtp/send", body, uuid.New(), gin.Params{{Key: "provider", Value: "smtp"}})
	h.SendEmail(c)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestSendEmail_MissingField_Returns400(t *testing.T) {
	h := &Handler{queries: &stubQuerier{}}
	body, _ := json.Marshal(map[string]string{"from": "x@y.com"}) // missing to/subject/body
	c, w := ginCtx("POST", "/email/smtp/send", body, uuid.New(), gin.Params{{Key: "provider", Value: "smtp"}})
	h.SendEmail(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing required fields, got %d", w.Code)
	}
}

func TestSendEmail_PayloadContainsProvider(t *testing.T) {
	// Verify the queued payload encodes the provider name so the worker can route it.
	var gotParams store.CreateJobParams
	q := &stubQuerier{
		createJobFn: func(_ context.Context, arg store.CreateJobParams) (store.Job, error) {
			gotParams = arg
			return store.Job{ID: uuid.New()}, nil
		},
	}
	h := &Handler{queries: q}

	body, _ := json.Marshal(map[string]interface{}{
		"to": []string{"a@b.com"}, "from": "x@y.com", "subject": "s", "body": "b",
	})
	c, w := ginCtx("POST", "/email/sendgrid/send", body, uuid.New(), gin.Params{{Key: "provider", Value: "sendgrid"}})
	h.SendEmail(c)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	var payload map[string]interface{}
	json.Unmarshal(gotParams.Payload, &payload)
	if payload["provider"] != "sendgrid" {
		t.Errorf("expected provider=sendgrid in payload, got %v", payload["provider"])
	}
}

// --- SendSMS tests ---

func TestSendSMS_Async_CreatesJobAndReturns202(t *testing.T) {
	tenantID := uuid.New()
	var gotParams store.CreateJobParams
	q := &stubQuerier{
		createJobFn: func(_ context.Context, arg store.CreateJobParams) (store.Job, error) {
			gotParams = arg
			return store.Job{ID: uuid.New(), Status: "pending"}, nil
		},
	}
	h := &Handler{queries: q}

	body, _ := json.Marshal(map[string]string{"from": "+15550001111", "to": "+15559998888", "body": "Hello"})
	c, w := ginCtx("POST", "/sms/twilio/send", body, tenantID, gin.Params{{Key: "provider", Value: "twilio"}})
	h.SendSMS(c)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	if gotParams.JobType != "sms.send" {
		t.Errorf("expected job_type=sms.send, got %s", gotParams.JobType)
	}
}

// --- GetJob tests ---

func TestGetJob_Found_Returns200(t *testing.T) {
	tenantID := uuid.New()
	jobID := uuid.New()
	now := time.Now()
	job := store.Job{
		ID:        jobID,
		TenantID:  tenantID,
		JobType:   "email.send",
		Status:    "completed",
		Error:     pgtype.Text{Valid: false},
		RunAt:     now,
		CreatedAt: now,
	}

	q := &stubQuerier{
		getJobFn: func(_ context.Context, arg store.GetJobParams) (store.Job, error) {
			if arg.ID != jobID || arg.TenantID != tenantID {
				t.Errorf("unexpected GetJob params: %+v", arg)
			}
			return job, nil
		},
	}
	h := &Handler{queries: q}

	c, w := ginCtx("GET", "/jobs/"+jobID.String(), nil, tenantID,
		gin.Params{{Key: "id", Value: jobID.String()}})
	h.GetJob(c)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", resp["status"])
	}
}

func TestGetJob_NotFound_Returns404(t *testing.T) {
	h := &Handler{queries: &stubQuerier{}} // getJobFn is nil → returns pgx.ErrNoRows

	jobID := uuid.New()
	c, w := ginCtx("GET", "/jobs/"+jobID.String(), nil, uuid.New(),
		gin.Params{{Key: "id", Value: jobID.String()}})
	h.GetJob(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetJob_InvalidUUID_Returns400(t *testing.T) {
	h := &Handler{queries: &stubQuerier{}}

	c, w := ginCtx("GET", "/jobs/not-a-uuid", nil, uuid.New(),
		gin.Params{{Key: "id", Value: "not-a-uuid"}})
	h.GetJob(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGetJob_TenantScoped(t *testing.T) {
	// A job belonging to another tenant must return 404.
	ownerTenant := uuid.New()
	callerTenant := uuid.New()
	jobID := uuid.New()

	q := &stubQuerier{
		getJobFn: func(_ context.Context, arg store.GetJobParams) (store.Job, error) {
			if arg.TenantID != ownerTenant {
				return store.Job{}, pgx.ErrNoRows
			}
			return store.Job{ID: jobID}, nil
		},
	}
	h := &Handler{queries: q}

	c, w := ginCtx("GET", "/jobs/"+jobID.String(), nil, callerTenant,
		gin.Params{{Key: "id", Value: jobID.String()}})
	h.GetJob(c)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for cross-tenant access, got %d", w.Code)
	}
}
