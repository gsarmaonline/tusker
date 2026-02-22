package oauth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// StatePayload is encoded into the OAuth state parameter.
type StatePayload struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	RedirectURI string    `json:"redirect_uri"`
	Nonce       string    `json:"nonce"`
}

// EncodeState encodes a StatePayload as a base64 JSON string.
func EncodeState(tenantID uuid.UUID, redirectURI string) (string, error) {
	nonce, err := generateNonce()
	if err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}
	payload := StatePayload{
		TenantID:    tenantID,
		RedirectURI: redirectURI,
		Nonce:       nonce,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// DecodeState decodes and returns the StatePayload from an OAuth callback state param.
func DecodeState(state string) (*StatePayload, error) {
	b, err := base64.URLEncoding.DecodeString(state)
	if err != nil {
		return nil, errors.New("invalid state encoding")
	}
	var payload StatePayload
	if err := json.Unmarshal(b, &payload); err != nil {
		return nil, errors.New("invalid state payload")
	}
	if payload.TenantID == uuid.Nil || payload.RedirectURI == "" || payload.Nonce == "" {
		return nil, errors.New("incomplete state payload")
	}
	return &payload, nil
}

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
