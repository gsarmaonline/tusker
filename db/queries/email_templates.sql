-- name: UpsertEmailTemplate :one
INSERT INTO email_templates (tenant_id, name, subject, body, html)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (tenant_id, name) DO UPDATE
    SET subject    = EXCLUDED.subject,
        body       = EXCLUDED.body,
        html       = EXCLUDED.html,
        updated_at = NOW()
RETURNING *;

-- name: GetEmailTemplate :one
SELECT * FROM email_templates
WHERE tenant_id = $1 AND name = $2;

-- name: ListEmailTemplates :many
SELECT * FROM email_templates
WHERE tenant_id = $1
ORDER BY name;

-- name: DeleteEmailTemplate :exec
DELETE FROM email_templates
WHERE tenant_id = $1 AND name = $2;
