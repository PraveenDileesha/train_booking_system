-- name: CreateTripFare :one
INSERT INTO trip_fares (trip_id, class, is_reservable, rate_per_km)
VALUES ($1, $2, $3, $4)
RETURNING id, trip_id, class, is_reservable, rate_per_km;

-- name: ListTripFares :many
SELECT id, trip_id, class, is_reservable, rate_per_km
FROM trip_fares
WHERE trip_id = $1
ORDER BY class, is_reservable;

-- name: GetTripFare :one
SELECT id, trip_id, class, is_reservable, rate_per_km
FROM trip_fares
WHERE trip_id = $1 AND class = $2 AND is_reservable = $3;
