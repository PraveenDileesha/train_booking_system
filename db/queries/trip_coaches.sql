-- name: CreateTripCoaches :exec
-- Position is derived from each coach's order in the coach_ids array.
INSERT INTO trip_coaches (trip_id, coach_id, position)
SELECT sqlc.arg(trip_id)::int, coach_id, (ordinality - 1)::int
FROM unnest(sqlc.arg(coach_ids)::int[]) WITH ORDINALITY AS t(coach_id, ordinality);

-- name: ListTripCoaches :many
SELECT tc.id, tc.trip_id, tc.coach_id, tc.position,
       c.coach_name, c.class, c.is_reservable, c.row_count, c.capacity
FROM trip_coaches tc
JOIN coaches c ON c.id = tc.coach_id
WHERE tc.trip_id = $1
ORDER BY tc.position;
