package tenant

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gsarma/tusker/internal/crypto"
	"github.com/gsarma/tusker/internal/store"
)

type Service struct {
	db      *pgxpool.Pool
	queries *store.Queries
	enc     *crypto.Encryptor
}

func NewService(db *pgxpool.Pool, enc *crypto.Encryptor) *Service {
	return &Service{
		db:      db,
		queries: store.New(db),
		enc:     enc,
	}
}

// Create provisions a new tenant, returning the raw API key (shown once).
func (s *Service) Create(ctx context.Context) (apiKey string, tenantID uuid.UUID, err error) {
	rawKey, err := generateAPIKey()
	if err != nil {
		return "", uuid.Nil, err
	}

	keyHash := hashAPIKey(rawKey)

	_, encDataKey, err := s.enc.GenerateDataKey()
	if err != nil {
		return "", uuid.Nil, err
	}

	t, err := s.queries.CreateTenant(ctx, store.CreateTenantParams{
		ApiKeyHash:       keyHash,
		EncryptedDataKey: encDataKey,
	})
	if err != nil {
		return "", uuid.Nil, err
	}

	return rawKey, t.ID, nil
}

// GetByAPIKey resolves a tenant from a raw API key.
func (s *Service) GetByAPIKey(ctx context.Context, rawKey string) (*store.Tenant, error) {
	hash := hashAPIKey(rawKey)
	t, err := s.queries.GetTenantByAPIKeyHash(ctx, hash)
	if err != nil {
		return nil, errors.New("invalid API key")
	}
	return &t, nil
}

// DataKey decrypts and returns the tenant's plaintext data key.
func (s *Service) DataKey(t *store.Tenant) ([]byte, error) {
	return s.enc.DecryptDataKey(t.EncryptedDataKey)
}

func hashAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

func generateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := randRead(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
