-- name: GetRulesByFlag :many
SELECT * FROM flag_rules 
WHERE flag_id = $1 
ORDER BY priority ASC;

-- name: CreateFlagRule :one
INSERT INTO flag_rules (flag_id, priority, rollout, variant) 
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: UpdateFlagRule :one
UPDATE flag_rules 
SET rollout = COALESCE($3, rollout),
    variant = COALESCE($4, variant)
WHERE flag_id = $1 AND priority = $2
RETURNING *;

-- name: DeleteFlagRule :exec
DELETE FROM flag_rules 
WHERE flag_id = $1 AND priority = $2;