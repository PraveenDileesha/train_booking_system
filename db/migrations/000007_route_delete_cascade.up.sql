-- route_versions and route_stations are a route's own composition data, not usage of it, so deleting a route cascades to them and cleans up its versions and stops.
-- trips.route_version_id stays ON DELETE RESTRICT. A route that still has trips scheduled against it cannot be deleted.

ALTER TABLE route_versions
    DROP CONSTRAINT route_versions_route_id_fkey,
    ADD CONSTRAINT route_versions_route_id_fkey
        FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE CASCADE;

ALTER TABLE route_stations
    DROP CONSTRAINT route_stations_route_version_id_fkey,
    ADD CONSTRAINT route_stations_route_version_id_fkey
        FOREIGN KEY (route_version_id) REFERENCES route_versions(id) ON DELETE CASCADE;
