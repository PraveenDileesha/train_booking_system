-- name: CreateSeats :copyfrom
INSERT INTO seats (coach_id, seat_number)
VALUES ($1, $2);

-- name: ListSeatsByCoach :many
SELECT id, coach_id, seat_number
FROM seats
WHERE coach_id = $1
ORDER BY id;
