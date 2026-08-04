-- Trips record both departure and arrival, so duration and overnight services are represented directly instead of derived from departure_time alone.
ALTER TABLE trips
    ADD COLUMN arrival_date DATE,
    ADD COLUMN arrival_time TIME;

-- Existing rows get a placeholder one-hour arrival so the strictly-after CHECK below can be added.
-- New trips always supply their own arrival.
UPDATE trips SET
    arrival_date = (departure_date + departure_time + interval '1 hour')::date,
    arrival_time = (departure_date + departure_time + interval '1 hour')::time;

ALTER TABLE trips
    ALTER COLUMN arrival_date SET NOT NULL,
    ALTER COLUMN arrival_time SET NOT NULL;

ALTER TABLE trips
    ADD CONSTRAINT arrival_after_departure
    CHECK ((arrival_date, arrival_time) > (departure_date, departure_time));
