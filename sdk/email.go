package tusker

import (
	"context"
	"fmt"
	"net/http"
)

// EmailService provides email configuration, sending, and template operations.
type EmailService struct {
	c *Client
}

// SetConfig stores the email provider configuration for this tenant.
// provider is one of "smtp" or "sendgrid".
// cfg must be one of SMTPConfig or SendGridConfig (marshalled to JSON).
func (s *EmailService) SetConfig(ctx context.Context, provider string, cfg any) error {
	path := fmt.Sprintf("/email/%s/config", provider)
	_, err := doRequest[StatusResponse](ctx, s.c, http.MethodPost, path, cfg, http.StatusOK)
	return err
}

// SendOptions controls how an email is sent.
type SendOptions struct {
	// Sync, when true, waits for the email to be delivered before returning.
	// By default emails are queued and sent asynchronously.
	Sync bool
}

// Send queues (or synchronously sends) an email via the given provider.
// provider is one of "smtp" or "sendgrid".
func (s *EmailService) Send(ctx context.Context, provider string, req SendEmailRequest, opts *SendOptions) (*SendEmailResponse, error) {
	path := fmt.Sprintf("/email/%s/send", provider)
	query := map[string]string{}
	if opts != nil && opts.Sync {
		query["sync"] = "true"
	}
	// Async returns 202, sync returns 200
	return doRequestWithQuery[SendEmailResponse](ctx, s.c, http.MethodPost, path, query, req,
		http.StatusAccepted, http.StatusOK)
}

// CreateTemplate creates or updates a named email template.
// Templates use Go text/template syntax. The optional HTML field uses html/template.
func (s *EmailService) CreateTemplate(ctx context.Context, req CreateEmailTemplateRequest) (*EmailTemplate, error) {
	return doRequest[EmailTemplate](ctx, s.c, http.MethodPost, "/email/templates", req, http.StatusOK)
}

// ListTemplates returns all available templates (built-in and custom).
func (s *EmailService) ListTemplates(ctx context.Context) ([]TemplateListItem, error) {
	result, err := doRequest[[]TemplateListItem](ctx, s.c, http.MethodGet, "/email/templates", nil, http.StatusOK)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// DeleteTemplate deletes a custom template by name.
func (s *EmailService) DeleteTemplate(ctx context.Context, name string) error {
	path := fmt.Sprintf("/email/templates/%s", name)
	_, err := doRequest[StatusResponse](ctx, s.c, http.MethodDelete, path, nil, http.StatusOK)
	return err
}

// SendTemplate renders a named template with variables and sends it.
// provider is one of "smtp" or "sendgrid".
func (s *EmailService) SendTemplate(ctx context.Context, provider string, req SendTemplateRequest) (*StatusResponse, error) {
	path := fmt.Sprintf("/email/%s/send-template", provider)
	return doRequest[StatusResponse](ctx, s.c, http.MethodPost, path, req, http.StatusOK)
}
