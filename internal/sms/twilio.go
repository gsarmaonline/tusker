package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// TwilioProvider sends SMS messages via the Twilio REST API.
type TwilioProvider struct {
	accountSID string
	authToken  string
}

// NewTwilioProvider creates a TwilioProvider for a specific tenant's credentials.
func NewTwilioProvider(accountSID, authToken string) *TwilioProvider {
	return &TwilioProvider{
		accountSID: accountSID,
		authToken:  authToken,
	}
}

func (t *TwilioProvider) Send(ctx context.Context, from, to, body string) (*Message, error) {
	endpoint := fmt.Sprintf(
		"https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json",
		t.accountSID,
	)

	form := url.Values{}
	form.Set("From", from)
	form.Set("To", to)
	form.Set("Body", body)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("twilio: build request: %w", err)
	}
	req.SetBasicAuth(t.accountSID, t.authToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("twilio: send request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		SID     string `json:"sid"`
		Status  string `json:"status"`
		Message string `json:"message"` // present on error
		Code    int    `json:"code"`    // Twilio error code on failure
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("twilio: decode response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("twilio: API error %d (code %d): %s",
			resp.StatusCode, result.Code, result.Message)
	}

	return &Message{SID: result.SID, Status: result.Status}, nil
}
