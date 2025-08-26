-- name: CreateExperimentVariant :one
INSERT INTO experiment_variants (experiment_id, name, weight, config) 
VALUES ($1, $2, $3, $4) 
RETURNING *;

-- name: GetVariantsByExperiment :many
SELECT * FROM experiment_variants 
WHERE experiment_id = $1 
ORDER BY name;

-- name: UpdateExperimentVariant :one
UPDATE experiment_variants 
SET weight = COALESCE($3, weight),
    config = COALESCE($4, config)
WHERE experiment_id = $1 AND name = $2
RETURNING *;