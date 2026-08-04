-- Only one version of a route can be active (bookable) at a time.
CREATE UNIQUE INDEX route_versions_one_active_per_route
    ON route_versions (route_id)
    WHERE is_active;
