-- name: CreateTenant :one
INSERT INTO tenants (api_key_hash, encrypted_data_key)
VALUES ($1, $2)
RETURNING *;

-- name: GetTenantByAPIKeyHash :one
SELECT * FROM tenants
WHERE api_key_hash = $1;

-- name: GetTenantByID :one
SELECT * FROM tenants
WHERE id = $1;
