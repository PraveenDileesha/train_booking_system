ALTER TABLE route_versions
    ALTER COLUMN version_no TYPE INTEGER USING version_no::integer;
