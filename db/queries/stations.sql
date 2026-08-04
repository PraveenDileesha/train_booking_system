-- name: CreateStation :one
INSERT INTO stations (name)
VALUES ($1)
RETURNING id, name;

-- name: ListStations :many
-- Numeric ID order for the admin table.
-- in_use marks stations referenced by route_stations, which therefore can't be deleted.
SELECT s.id, s.name,
       EXISTS (SELECT 1 FROM route_stations rs WHERE rs.station_id = s.id) AS in_use
FROM stations s
ORDER BY s.id
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: ListStationsByName :many
-- Alphabetical order for every station picker outside the admin table.
SELECT s.id, s.name,
       EXISTS (SELECT 1 FROM route_stations rs WHERE rs.station_id = s.id) AS in_use
FROM stations s
ORDER BY s.name
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountStations :one
SELECT COUNT(*) FROM stations;

-- name: GetStation :one
SELECT id, name
FROM stations
WHERE id = $1;

-- name: DeleteStation :execrows
DELETE FROM stations
WHERE id = $1;