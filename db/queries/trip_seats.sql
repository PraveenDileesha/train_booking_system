-- name: CreateTripSeatsForCoaches :exec
INSERT INTO trip_seats (trip_id, seat_id)
SELECT sqlc.arg(trip_id)::int, s.id
FROM seats s
WHERE s.coach_id = ANY(sqlc.arg(coach_ids)::int[]);

-- name: ListTripSeatsWithAvailability :many
-- is_available is false only for a CONFIRMED booking or a live (unexpired) PENDING hold whose range overlaps [start_sequence, end_sequence).
SELECT
    ts.id AS trip_seat_id,
    s.id AS seat_id,
    s.seat_number,
    c.id AS coach_id,
    c.coach_name,
    c.class,
    c.is_reservable,
    tc.position AS coach_position,
    NOT EXISTS (
        SELECT 1 FROM bookings b
        WHERE b.trip_seat_id = ts.id
          AND (b.status = 'CONFIRMED' OR (b.status = 'PENDING' AND b.held_until >= now()))
          AND b.seg && int4range(sqlc.arg(start_sequence)::int, sqlc.arg(end_sequence)::int, '[)')
    ) AS is_available
FROM trip_seats ts
JOIN seats s ON s.id = ts.seat_id
JOIN coaches c ON c.id = s.coach_id
JOIN trip_coaches tc ON tc.trip_id = ts.trip_id AND tc.coach_id = c.id
WHERE ts.trip_id = sqlc.arg(trip_id)
ORDER BY tc.position, s.id;

-- name: GetTripSeat :one
-- Includes route_version_id, used to resolve station sequences and distances.
SELECT ts.id, ts.trip_id, ts.seat_id, s.seat_number, c.id AS coach_id, c.coach_name, c.class, c.is_reservable,
       t.route_version_id
FROM trip_seats ts
JOIN seats s ON s.id = ts.seat_id
JOIN coaches c ON c.id = s.coach_id
JOIN trips t ON t.id = ts.trip_id
WHERE ts.id = $1;
