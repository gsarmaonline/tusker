package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SendGridConfig holds credentials for the SendGrid API.
type SendGridConfig struct {
	APIKey string `json:"api_key"`
}

// SendGridProvider sends email via the SendGrid v3 Mail Send API.
type SendGridProvider struct {
	cfg    SendGridConfig
	client *http.Client
}

func NewSendGridProvider(cfg SendGridConfig) *SendGridProvider {
	return &SendGridProvider{cfg: cfg, client: http.DefaultClient}
}

func (p *SendGridProvider) Send(ctx context.Context, msg Message) error {
	toList := make([]map[string]string, len(msg.To))
	for i, addr := range msg.To {
		toList[i] = map[string]string{"email": addr}
	}

	contentType := "text/plain"
	if msg.HTML {
		contentType = "text/html"
	}

	payload := map[string]any{
		"personalizations": []map[string]any{
			{"to": toList},
		},
		"from":    map[string]string{"email": msg.From},
		"subject": msg.Subject,
		"content": []map[string]string{
			{"type": contentType, "value": msg.Body},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal sendgrid payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build sendgrid request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("sendgrid request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("sendgrid returned status %d", resp.StatusCode)
	}
	return nil
}
