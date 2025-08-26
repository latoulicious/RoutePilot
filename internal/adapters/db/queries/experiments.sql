-- name: CreateExperiment :one
INSERT INTO experiments (tenant_id, key, flag_id, traffic) 
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: GetExperimentByTenantKey :one
SELECT * FROM experiments 
WHERE tenant_id = $1 AND key = $2;

-- name: GetExperimentByFlagID :one
SELECT * FROM experiments 
WHERE flag_id = $1 AND status = 'running';

-- name: UpdateExperimentStatus :one
UPDATE experiments 
SET status = $3
WHERE tenant_id = $1 AND key = $2
RETURNING *;

-- name: ListExperimentsByTenant :many
SELECT * FROM experiments 
WHERE tenant_id = $1 
ORDER BY created_at DESC;

-- name: GetExperimentByID :one
SELECT * FROM experiments 
WHERE id = $1;