-- name: UpsertProviderConfig :one
INSERT INTO oauth_provider_configs (tenant_id, provider, client_id, encrypted_client_secret)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, provider) DO UPDATE
    SET client_id = EXCLUDED.client_id,
        encrypted_client_secret = EXCLUDED.encrypted_client_secret
RETURNING *;

-- name: GetProviderConfig :one
SELECT * FROM oauth_provider_configs
WHERE tenant_id = $1 AND provider = $2;

-- name: UpsertOAuthToken :one
INSERT INTO oauth_tokens (tenant_id, provider, user_id, encrypted_access_token, encrypted_refresh_token, expires_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (tenant_id, provider, user_id) DO UPDATE
    SET encrypted_access_token  = EXCLUDED.encrypted_access_token,
        encrypted_refresh_token = EXCLUDED.encrypted_refresh_token,
        expires_at              = EXCLUDED.expires_at,
        updated_at              = NOW()
RETURNING *;

-- name: GetOAuthToken :one
SELECT * FROM oauth_tokens
WHERE tenant_id = $1 AND provider = $2 AND user_id = $3;

-- name: DeleteOAuthToken :exec
DELETE FROM oauth_tokens
WHERE tenant_id = $1 AND provider = $2 AND user_id = $3;
