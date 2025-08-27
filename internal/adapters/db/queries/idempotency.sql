-- name: GetIdempotencyKey :one
SELECT * FROM idempotency_keys 
WHERE tenant_id = $1 AND key = $2;

-- name: CreateIdempotencyKey :one
INSERT INTO idempotency_keys (tenant_id, key, method, path_hash, status) 
VALUES ($1, $2, $3, $4, $5) 
RETURNING *;

-- name: UpdateIdempotencyStatus :exec
UPDATE idempotency_keys
SET status = $3
WHERE tenant_id = $1 AND key = $2;

-- name: CleanupOldIdempotencyKeys :exec
DELETE FROM idempotency_keys 
WHERE created_at < NOW() - INTERVAL '24 hours';
