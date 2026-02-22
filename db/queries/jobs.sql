-- name: CreateJob :one
INSERT INTO jobs (tenant_id, job_type, payload)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ClaimNextJob :one
UPDATE jobs SET
    status = 'running',
    started_at = NOW(),
    attempt = attempt + 1
WHERE id = (
    SELECT id FROM jobs
    WHERE status = 'pending' AND run_at <= NOW()
    ORDER BY run_at ASC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: UpdateJobStatus :one
UPDATE jobs SET
    status = $2,
    error = $3,
    completed_at = $4,
    run_at = $5
WHERE id = $1
RETURNING *;

-- name: GetJob :one
SELECT * FROM jobs
WHERE id = $1 AND tenant_id = $2;
