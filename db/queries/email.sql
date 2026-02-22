-- name: UpsertEmailProviderConfig :one
INSERT INTO email_provider_configs (tenant_id, provider, encrypted_config)
VALUES ($1, $2, $3)
ON CONFLICT (tenant_id, provider) DO UPDATE
    SET encrypted_config = EXCLUDED.encrypted_config
RETURNING *;

-- name: GetEmailProviderConfig :one
SELECT * FROM email_provider_configs
WHERE tenant_id = $1 AND provider = $2;
