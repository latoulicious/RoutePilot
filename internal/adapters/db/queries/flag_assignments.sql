-- name: GetAssignment :one
SELECT * FROM flag_assignments 
WHERE flag_id = $1 AND subject_id = $2;

-- name: UpsertAssignment :one
INSERT INTO flag_assignments (flag_id, subject_id, bucket, chosen_variant) 
VALUES ($1, $2, $3, $4)
ON CONFLICT (flag_id, subject_id) 
DO UPDATE SET 
    bucket = EXCLUDED.bucket,
    chosen_variant = EXCLUDED.chosen_variant,
    assigned_at = NOW()
RETURNING *;