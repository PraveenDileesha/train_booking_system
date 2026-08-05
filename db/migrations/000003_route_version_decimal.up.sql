-- Route versions now use decimal numbering (1.0, 1.1, ... 1.9, 2.0, ...), computed automatically by the API and never entered by an admin directly.
ALTER TABLE route_versions
    ALTER COLUMN version_no TYPE NUMERIC(4, 1) USING version_no::numeric(4, 1);
