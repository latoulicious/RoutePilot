-- name: AddOutbox :one
INSERT INTO outbox (tenant_id, topic, key, payload) 
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: ClaimOutboxBatch :many
SELECT * FROM outbox 
WHERE published_at IS NULL 
ORDER BY created_at ASC 
LIMIT $1 
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE outbox 
SET published_at = NOW() 
WHERE id = ANY($1::uuid[]);