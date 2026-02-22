package tusker

import (
	"context"
	"fmt"
	"net/http"
)

// SMSService provides SMS configuration and sending operations.
type SMSService struct {
	c *Client
}

// SetConfig stores the SMS provider configuration for this tenant.
// provider is currently "twilio".
// cfg must be a TwilioConfig (marshalled to JSON).
func (s *SMSService) SetConfig(ctx context.Context, provider string, cfg any) error {
	path := fmt.Sprintf("/sms/%s/config", provider)
	_, err := doRequest[StatusResponse](ctx, s.c, http.MethodPost, path, cfg, http.StatusOK)
	return err
}

// Send queues (or synchronously sends) an SMS via the given provider.
// provider is currently "twilio".
func (s *SMSService) Send(ctx context.Context, provider string, req SendSMSRequest, opts *SendOptions) (*SendSMSResponse, error) {
	path := fmt.Sprintf("/sms/%s/send", provider)
	query := map[string]string{}
	if opts != nil && opts.Sync {
		query["sync"] = "true"
	}
	// Async returns 202, sync returns 200
	return doRequestWithQuery[SendSMSResponse](ctx, s.c, http.MethodPost, path, query, req,
		http.StatusAccepted, http.StatusOK)
}
