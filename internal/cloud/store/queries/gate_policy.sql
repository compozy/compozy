-- name: GetGatePolicy :one
SELECT mode, until, message FROM gate_policy WHERE id = 1;

-- name: UpdateGatePolicy :execrows
UPDATE gate_policy SET mode = $1, until = $2, message = $3, updated_by = $4, updated_at = $5
WHERE id = 1 AND (mode, until, message) IS DISTINCT FROM ($1, $2, $3);
