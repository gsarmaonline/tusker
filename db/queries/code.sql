-- name: UpsertCodeProviderConfig :one
INSERT INTO code_provider_configs (tenant_id, provider, encrypted_config)
VALUES ($1, $2, $3)
ON CONFLICT (tenant_id, provider) DO UPDATE
    SET encrypted_config = EXCLUDED.encrypted_config
RETURNING *;

-- name: GetCodeProviderConfig :one
SELECT * FROM code_provider_configs
WHERE tenant_id = $1 AND provider = $2;

-- name: InsertCodeExecution :one
INSERT INTO code_executions (job_id, tenant_id, stdout, stderr, compile_output, status, exec_time, memory)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetCodeExecution :one
SELECT * FROM code_executions
WHERE job_id = $1 AND tenant_id = $2;
