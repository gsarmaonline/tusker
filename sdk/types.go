package tusker

import "time"

// --- Tenant ---

// CreateTenantResponse is returned when a new tenant is provisioned.
type CreateTenantResponse struct {
	TenantID string `json:"tenant_id"`
	APIKey   string `json:"api_key"`
	Note     string `json:"note"`
}

// HealthResponse is returned by the /health endpoint.
type HealthResponse struct {
	Status string `json:"status"`
}

// --- OAuth ---

// SetOAuthConfigRequest sets the OAuth client credentials for a provider.
type SetOAuthConfigRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

// GetOAuthTokenResponse is returned by GET /oauth/:provider/token.
type GetOAuthTokenResponse struct {
	AccessToken string     `json:"access_token"`
	Provider    string     `json:"provider"`
	UserID      string     `json:"user_id"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// StatusResponse is a generic {"status": "..."} response.
type StatusResponse struct {
	Status string `json:"status"`
}

// --- Email ---

// SMTPConfig holds SMTP provider configuration.
type SMTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// SendGridConfig holds SendGrid provider configuration.
type SendGridConfig struct {
	APIKey string `json:"api_key"`
}

// SendEmailRequest sends an email via a configured provider.
type SendEmailRequest struct {
	To      []string `json:"to"`
	From    string   `json:"from"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	HTML    bool     `json:"html"`
}

// SendEmailResponse is returned by POST /email/:provider/send.
// When async (default), JobID and Status are populated.
// When sync (?sync=true), only Status is populated.
type SendEmailResponse struct {
	JobID  string `json:"job_id,omitempty"`
	Status string `json:"status"`
}

// CreateEmailTemplateRequest creates or updates a named email template.
type CreateEmailTemplateRequest struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	HTML    string `json:"html,omitempty"`
}

// EmailTemplate is the stored representation of a template.
type EmailTemplate struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Name      string    `json:"name"`
	Subject   string    `json:"subject"`
	Body      string    `json:"body"`
	HTML      string    `json:"html"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TemplateListItem is a template entry returned by GET /email/templates.
type TemplateListItem struct {
	Name    string `json:"name"`
	Subject string `json:"subject"`
	Body    string `json:"body"`
	HTML    string `json:"html"`
	Custom  bool   `json:"custom"`
}

// SendTemplateRequest renders a template with variables and sends it.
type SendTemplateRequest struct {
	Template  string            `json:"template"`
	To        []string          `json:"to"`
	From      string            `json:"from"`
	Variables map[string]string `json:"variables,omitempty"`
}

// --- SMS ---

// TwilioConfig holds Twilio provider configuration.
type TwilioConfig struct {
	AccountSID string `json:"account_sid"`
	AuthToken  string `json:"auth_token"`
}

// SendSMSRequest sends an SMS message via a configured provider.
type SendSMSRequest struct {
	From string `json:"from"`
	To   string `json:"to"`
	Body string `json:"body"`
}

// SendSMSResponse is returned by POST /sms/:provider/send.
// When async (default), JobID and Status are populated.
// When sync (?sync=true), MessageSID, Status, From, and To are populated.
type SendSMSResponse struct {
	JobID      string `json:"job_id,omitempty"`
	Status     string `json:"status"`
	MessageSID string `json:"message_sid,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

// --- Jobs ---

// Job represents an async background job.
type Job struct {
	ID          string     `json:"id"`
	TenantID    string     `json:"tenant_id"`
	JobType     string     `json:"job_type"`
	Status      string     `json:"status"`
	Attempt     int        `json:"attempt"`
	MaxAttempts int        `json:"max_attempts"`
	Error       *string    `json:"error,omitempty"`
	RunAt       time.Time  `json:"run_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

// JobStatus constants for Job.Status.
const (
	JobStatusPending   = "pending"
	JobStatusRunning   = "running"
	JobStatusCompleted = "completed"
	JobStatusFailed    = "failed"
)
