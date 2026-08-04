ALTER TABLE route_stations
    DROP CONSTRAINT route_stations_route_version_id_fkey,
    ADD CONSTRAINT route_stations_route_version_id_fkey
        FOREIGN KEY (route_version_id) REFERENCES route_versions(id) ON DELETE RESTRICT;

ALTER TABLE route_versions
    DROP CONSTRAINT route_versions_route_id_fkey,
    ADD CONSTRAINT route_versions_route_id_fkey
        FOREIGN KEY (route_id) REFERENCES routes(id) ON DELETE RESTRICT;
