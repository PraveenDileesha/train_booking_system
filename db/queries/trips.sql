-- name: CreateTrip :one
INSERT INTO trips (route_version_id, departure_date, departure_time, arrival_date, arrival_time, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, route_version_id, departure_date, departure_time, status, arrival_date, arrival_time;

-- name: GetTrip :one
SELECT id, route_version_id, departure_date, departure_time, status, arrival_date, arrival_time
FROM trips
WHERE id = $1;

-- name: ListTrips :many
SELECT t.id, t.route_version_id, t.departure_date, t.departure_time, t.status, t.arrival_date, t.arrival_time,
       r.id AS route_id, r.name AS route_name
FROM trips t
JOIN route_versions rv ON rv.id = t.route_version_id
JOIN routes r ON r.id = rv.route_id
ORDER BY t.departure_date, t.departure_time
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountTrips :one
SELECT COUNT(*) FROM trips;

-- name: SearchTrips :many
-- Matches by station pair, not route_id. A trip qualifies if its route passes through both stations in that order on the given date.
SELECT t.id, t.route_version_id, t.departure_date, t.departure_time, t.status, t.arrival_date, t.arrival_time,
       r.name AS route_name
FROM trips t
JOIN route_versions rv ON rv.id = t.route_version_id
JOIN routes r ON r.id = rv.route_id
JOIN route_stations rs_start ON rs_start.route_version_id = t.route_version_id
    AND rs_start.station_id = sqlc.arg(start_station_id)
JOIN route_stations rs_end ON rs_end.route_version_id = t.route_version_id
    AND rs_end.station_id = sqlc.arg(end_station_id)
WHERE t.departure_date = sqlc.arg(departure_date)
  AND rs_start.stop_sequence < rs_end.stop_sequence
ORDER BY t.departure_time
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountSearchTrips :one
SELECT COUNT(*)
FROM trips t
JOIN route_versions rv ON rv.id = t.route_version_id
JOIN routes r ON r.id = rv.route_id
JOIN route_stations rs_start ON rs_start.route_version_id = t.route_version_id
    AND rs_start.station_id = sqlc.arg(start_station_id)
JOIN route_stations rs_end ON rs_end.route_version_id = t.route_version_id
    AND rs_end.station_id = sqlc.arg(end_station_id)
WHERE t.departure_date = sqlc.arg(departure_date)
  AND rs_start.stop_sequence < rs_end.stop_sequence;

-- name: DeleteTrip :execrows
-- Fails with a restrict_violation if any booking or unreserved ticket references this trip. Bookings RESTRICT trip_seats, and unreserved_tickets RESTRICTs trips directly.
DELETE FROM trips
WHERE id = $1;
