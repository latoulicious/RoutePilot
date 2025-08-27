-- name: CreateAPIKey :one
INSERT INTO api_keys (tenant_id, name, key_id, secret_hash, secret_enc) 
VALUES ($1, $2, $3, $4, $5) 
RETURNING *;

-- name: GetAPIKey :one
SELECT * FROM api_keys 
WHERE id = $1 AND active = TRUE;

-- name: GetAPIKeyByTenant :one
SELECT * FROM api_keys 
WHERE tenant_id = $1 AND id = $2 AND active = TRUE;

-- name: UpdateAPIKeyLastUsed :exec
UPDATE api_keys 
SET last_used_at = NOW() 
WHERE id = $1;

-- name: GetAPIKeyByKeyID :one
SELECT * FROM api_keys 
WHERE key_id = $1 AND active = TRUE;