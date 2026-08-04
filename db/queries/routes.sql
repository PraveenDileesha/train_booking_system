-- name: CreateRoute :one
INSERT INTO routes (name)
VALUES ($1)
RETURNING id, name;

-- name: CreateRouteVersion :one
INSERT INTO route_versions (route_id, version_no, is_active)
VALUES ($1, $2, $3)
RETURNING id, route_id, version_no, created_at, is_active;

-- name: CreateRouteStation :one
INSERT INTO route_stations (route_version_id, station_id, stop_sequence, distance_from_origin)
VALUES ($1, $2, $3, $4)
RETURNING id, route_version_id, station_id, stop_sequence, distance_from_origin;

-- name: ListRoutes :many
SELECT id, name
FROM routes
ORDER BY name
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountRoutes :one
SELECT COUNT(*) FROM routes;

-- name: GetRouteVersionStations :many
SELECT rs.id, rs.station_id, s.name AS station_name, rs.stop_sequence, rs.distance_from_origin
FROM route_stations rs
JOIN stations s ON s.id = rs.station_id
WHERE rs.route_version_id = $1
ORDER BY rs.stop_sequence;

-- name: GetRoute :one
SELECT id, name
FROM routes
WHERE id = $1;

-- name: DeleteRoute :execrows
DELETE FROM routes
WHERE id = $1;

-- name: GetActiveRouteVersion :one
SELECT id, route_id, version_no, created_at, is_active
FROM route_versions
WHERE route_id = $1 AND is_active = true;

-- name: DeactivateRouteVersion :exec
UPDATE route_versions
SET is_active = false
WHERE id = $1;