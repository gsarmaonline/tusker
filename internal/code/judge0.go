package code

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Judge0Config holds the connection settings for a Judge0 CE instance.
// URL is the base URL of the Judge0 server (e.g. "http://judge0-server:2358").
// AuthToken is optional; send it as X-Auth-Token when AUTHN_TOKEN is configured.
type Judge0Config struct {
	URL       string `json:"url"`
	AuthToken string `json:"auth_token,omitempty"`
}

// Judge0Provider calls the Judge0 CE REST API to execute source code.
type Judge0Provider struct {
	url       string
	authToken string
	client    *http.Client
}

// NewJudge0Provider constructs a Judge0Provider from the given config.
func NewJudge0Provider(cfg Judge0Config) *Judge0Provider {
	return &Judge0Provider{
		url:       strings.TrimRight(cfg.URL, "/"),
		authToken: cfg.AuthToken,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Execute submits source code to Judge0 and waits synchronously for the result.
// Source code and stdin are base64-encoded in the request; Judge0 returns
// stdout/stderr as base64 which we decode before returning.
func (p *Judge0Provider) Execute(ctx context.Context, sourceCode string, languageID int, stdin string) (*Submission, error) {
	reqBody := map[string]interface{}{
		"source_code": base64.StdEncoding.EncodeToString([]byte(sourceCode)),
		"language_id": languageID,
	}
	if stdin != "" {
		reqBody["stdin"] = base64.StdEncoding.EncodeToString([]byte(stdin))
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		p.url+"/submissions?base64_encoded=true&wait=true", bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.authToken != "" {
		req.Header.Set("X-Auth-Token", p.authToken)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("submit to judge0: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("judge0 returned HTTP %d", resp.StatusCode)
	}

	var raw struct {
		Token         string  `json:"token"`
		Stdout        *string `json:"stdout"`
		Stderr        *string `json:"stderr"`
		CompileOutput *string `json:"compile_output"`
		Time          *string `json:"time"`
		Memory        *int    `json:"memory"`
		Status        struct {
			Description string `json:"description"`
		} `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode judge0 response: %w", err)
	}

	sub := &Submission{
		Token:  raw.Token,
		Status: raw.Status.Description,
	}
	if raw.Stdout != nil {
		if dec, err := base64.StdEncoding.DecodeString(*raw.Stdout); err == nil {
			sub.Stdout = string(dec)
		}
	}
	if raw.Stderr != nil {
		if dec, err := base64.StdEncoding.DecodeString(*raw.Stderr); err == nil {
			sub.Stderr = string(dec)
		}
	}
	if raw.CompileOutput != nil {
		if dec, err := base64.StdEncoding.DecodeString(*raw.CompileOutput); err == nil {
			sub.CompileOutput = string(dec)
		}
	}
	if raw.Time != nil {
		sub.Time = *raw.Time
	}
	if raw.Memory != nil {
		sub.Memory = *raw.Memory
	}
	return sub, nil
}
