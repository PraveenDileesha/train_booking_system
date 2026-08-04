ALTER TABLE bookings
    DROP CONSTRAINT no_overlapping_segments;

ALTER TABLE bookings
    ADD CONSTRAINT no_overlapping_segments
    EXCLUDE USING gist (trip_seat_id WITH =, seg WITH &&)
    WHERE (status = 'CONFIRMED')
    DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE bookings
    DROP COLUMN held_until;

DROP TABLE trip_fares;

ALTER TABLE coaches
    DROP COLUMN row_count,
    DROP COLUMN class;

DROP TYPE coach_class;
