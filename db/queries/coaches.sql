-- name: CreateCoach :one
INSERT INTO coaches (coach_name, class, is_reservable, row_count, capacity)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, coach_name, class, is_reservable, row_count, capacity;

-- name: ListCoaches :many
SELECT id, coach_name, class, is_reservable, row_count, capacity
FROM coaches
ORDER BY coach_name
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountCoaches :one
SELECT COUNT(*) FROM coaches;

-- name: GetCoach :one
SELECT id, coach_name, class, is_reservable, row_count, capacity
FROM coaches
WHERE id = $1;

-- name: DeleteCoach :execrows
DELETE FROM coaches
WHERE id = $1;
