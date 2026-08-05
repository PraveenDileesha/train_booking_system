-- Intermediate stops only. The trip's origin departure and destination arrival stay on trips.departure_time and trips.arrival_time, not duplicated here.
CREATE TABLE trip_stop_times (
    id                SERIAL PRIMARY KEY,
    trip_id           INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    route_station_id  INTEGER NOT NULL REFERENCES route_stations(id) ON DELETE RESTRICT,
    arrival_time      TIMESTAMP NOT NULL,
    departure_time    TIMESTAMP NOT NULL,
    UNIQUE (trip_id, route_station_id)
);

CREATE INDEX idx_trip_stop_times_trip ON trip_stop_times(trip_id);
