-- name: CreateCoach :one
INSERT INTO coaches (coach_name, class, is_reservable, row_count, capacity)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, coach_name, class, is_reservable, row_count, capacity;

-- name: ListCoaches :many
-- has_activity mirrors DeleteCoach's own restrict check, trip_coaches.coach_id, so the admin UI can hide the delete option for a coach that would fail anyway.
SELECT c.id, c.coach_name, c.class, c.is_reservable, c.row_count, c.capacity,
       EXISTS (SELECT 1 FROM trip_coaches tc WHERE tc.coach_id = c.id) AS has_activity
FROM coaches c
ORDER BY c.coach_name
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
