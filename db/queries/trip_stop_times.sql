-- name: CreateTripStopTimes :copyfrom
INSERT INTO trip_stop_times (trip_id, route_station_id, arrival_time, departure_time)
VALUES ($1, $2, $3, $4);

-- name: ListTripStopTimes :many
SELECT tst.route_station_id, rs.station_id, s.name AS station_name, rs.stop_sequence, rs.distance_from_origin,
       tst.arrival_time, tst.departure_time
FROM trip_stop_times tst
JOIN route_stations rs ON rs.id = tst.route_station_id
JOIN stations s ON s.id = rs.station_id
WHERE tst.trip_id = $1
ORDER BY rs.stop_sequence;
