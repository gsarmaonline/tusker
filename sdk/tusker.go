// Package tusker provides a Go client for the Tusker API.
//
// Tusker is a SaaS platform that provides a single unified API for common
// backend integrations (OAuth, email, SMS, workers, etc.).
//
// Usage:
//
//	client := tusker.New("https://api.tusker.io", "your-api-key")
//
//	// Create a tenant (no API key required)
//	provisioner := tusker.NewProvisioner("https://api.tusker.io")
//	tenant, err := provisioner.CreateTenant(ctx)
//
//	// Send an email
//	resp, err := client.Email.Send(ctx, "smtp", tusker.SendEmailRequest{
//	    To:      []string{"user@example.com"},
//	    From:    "no-reply@myapp.com",
//	    Subject: "Hello",
//	    Body:    "World",
//	})
package tusker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client is the authenticated Tusker API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client

	// Service accessors
	OAuth  *OAuthService
	Email  *EmailService
	SMS    *SMSService
	Jobs   *JobsService
}

// Provisioner is an unauthenticated client used only for tenant provisioning.
type Provisioner struct {
	baseURL    string
	httpClient *http.Client
}

// Option configures a Client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(hc *http.Client) Option {
	return func(c *Client) {
		c.httpClient = hc
	}
}

// New creates an authenticated Tusker client.
// baseURL should be the root URL (e.g. "https://api.tusker.io").
// apiKey is the Bearer token returned when a tenant is provisioned.
func New(baseURL, apiKey string, opts ...Option) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
	for _, o := range opts {
		o(c)
	}
	c.OAuth = &OAuthService{c: c}
	c.Email = &EmailService{c: c}
	c.SMS = &SMSService{c: c}
	c.Jobs = &JobsService{c: c}
	return c
}

// NewProvisioner creates an unauthenticated client for tenant provisioning.
func NewProvisioner(baseURL string, opts ...Option) *Provisioner {
	p := &Provisioner{
		baseURL:    strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{},
	}
	// Apply any HTTP client options via a temporary Client to reuse Option type
	tmp := &Client{}
	for _, o := range opts {
		o(tmp)
	}
	if tmp.httpClient != nil {
		p.httpClient = tmp.httpClient
	}
	return p
}

// CreateTenant provisions a new tenant and returns the API key.
// Store the returned API key securely — it is shown only once.
func (p *Provisioner) CreateTenant(ctx context.Context) (*CreateTenantResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/tenants", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseError(resp)
	}

	var out CreateTenantResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Health checks that the Tusker server is reachable and healthy.
func (c *Client) Health(ctx context.Context) (*HealthResponse, error) {
	return doRequest[HealthResponse](ctx, c, http.MethodGet, "/health", nil, http.StatusOK)
}

// --- internal helpers ---

func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("tusker: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	return req, nil
}

func doRequest[T any](ctx context.Context, c *Client, method, path string, body any, expectedStatus int) (*T, error) {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		return nil, parseError(resp)
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("tusker: decode response: %w", err)
	}
	return &out, nil
}

func doRequestWithQuery[T any](ctx context.Context, c *Client, method, path string, query map[string]string, body any, expectedStatuses ...int) (*T, error) {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}

	if len(query) > 0 {
		q := req.URL.Query()
		for k, v := range query {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	for _, s := range expectedStatuses {
		if resp.StatusCode == s {
			var out T
			if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
				return nil, fmt.Errorf("tusker: decode response: %w", err)
			}
			return &out, nil
		}
	}
	return nil, parseError(resp)
}

func parseError(resp *http.Response) *APIError {
	e := &APIError{StatusCode: resp.StatusCode}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err == nil && body.Error != "" {
		e.Message = body.Error
	} else {
		e.Message = http.StatusText(resp.StatusCode)
	}
	return e
}
