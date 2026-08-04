-- Coach class (drives seat layout and default fare multipliers)
CREATE TYPE coach_class AS ENUM ('FIRST_AC', 'SECOND', 'THIRD');

ALTER TABLE coaches
    ADD COLUMN class coach_class NOT NULL DEFAULT 'SECOND',
    ADD COLUMN row_count INTEGER NOT NULL DEFAULT 0;

ALTER TABLE coaches ALTER COLUMN class DROP DEFAULT;
ALTER TABLE coaches ALTER COLUMN row_count DROP DEFAULT;

-- ---------- Trip Fares ----------
-- Per-trip, per-class, per-(un)reserved Rs/km rate.
-- Entered by the admin when scheduling a trip so pricing can vary by season/train without touching global config.
CREATE TABLE trip_fares (
    id              SERIAL PRIMARY KEY,
    trip_id         INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    class           coach_class NOT NULL,
    is_reservable   BOOLEAN NOT NULL,
    rate_per_km     DECIMAL(10, 2) NOT NULL CHECK (rate_per_km > 0),
    UNIQUE (trip_id, class, is_reservable)
);

-- ---------- Booking holds ----------
-- A PENDING booking reserves the seat until held_until; after that it is treated as free by availability checks.
ALTER TABLE bookings
    ADD COLUMN held_until TIMESTAMP;

ALTER TABLE bookings
    DROP CONSTRAINT no_overlapping_segments;

-- Blocks a live hold from overlapping another PENDING or CONFIRMED booking on the same seat.
ALTER TABLE bookings
    ADD CONSTRAINT no_overlapping_segments
    EXCLUDE USING gist (trip_seat_id WITH =, seg WITH &&)
    WHERE (status IN ('PENDING', 'CONFIRMED'))
    DEFERRABLE INITIALLY IMMEDIATE;
