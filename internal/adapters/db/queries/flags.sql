-- name: GetFlagByTenantKey :one
SELECT id, tenant_id, key, description, type, enabled, salt, created_at, updated_at
FROM flags 
WHERE tenant_id = $1 AND key = $2;

-- name: CreateFlag :one
INSERT INTO flags (tenant_id, key, description, type, enabled, salt) 
VALUES ($1, $2, $3, $4, $5, $6) 
RETURNING *;

-- name: UpdateFlag :one
UPDATE flags 
SET enabled = COALESCE($3, enabled),
    salt = COALESCE($4, salt),
    description = COALESCE($5, description)
WHERE tenant_id = $1 AND key = $2
RETURNING *;

-- name: ListFlagsByTenant :many
SELECT * FROM flags 
WHERE tenant_id = $1 
ORDER BY created_at DESC;