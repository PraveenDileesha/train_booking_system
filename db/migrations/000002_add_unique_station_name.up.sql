-- Prevent duplicate station names (e.g. two "Colombo Fort" rows from accidental double-entry).
ALTER TABLE stations
    ADD CONSTRAINT unique_station_name UNIQUE (name);