ALTER TABLE trips
    DROP CONSTRAINT arrival_after_departure;

ALTER TABLE trips
    DROP COLUMN arrival_date,
    DROP COLUMN arrival_time;
