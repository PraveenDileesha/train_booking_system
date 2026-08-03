-- =========================================================
-- Segment-Based Train Seat Booking System — Schema
-- =========================================================

-- Needed for the GiST exclusion constraint (equality + range together)
CREATE EXTENSION IF NOT EXISTS btree_gist;

-- ---------- Enums ----------
CREATE TYPE trip_status AS ENUM ('SCHEDULED', 'BOARDING', 'DEPARTED', 'COMPLETED', 'CANCELLED');
CREATE TYPE booking_status AS ENUM ('PENDING', 'CONFIRMED', 'CANCELLED', 'EXPIRED');

-- ---------- Routes ----------
-- A named rail line (e.g. "Colombo Fort - Badulla")
CREATE TABLE routes (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(255) NOT NULL
);

-- ---------- Route Versions ----------
-- Immutable snapshot of a route's station list. Editing a route
-- creates a new version rather than mutating an existing one.
CREATE TABLE route_versions (
    id          SERIAL PRIMARY KEY,
    route_id    INTEGER NOT NULL REFERENCES routes(id) ON DELETE RESTRICT,
    version_no  INTEGER NOT NULL,
    created_at  TIMESTAMP NOT NULL DEFAULT now(),
    is_active   BOOLEAN NOT NULL DEFAULT true,
    UNIQUE (route_id, version_no)
);

-- ---------- Stations ----------
CREATE TABLE stations (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(255) NOT NULL
);

-- ---------- Route Stations ----------
-- Ordered list of stops for a given route version.
CREATE TABLE route_stations (
    id                      SERIAL PRIMARY KEY,
    route_version_id        INTEGER NOT NULL REFERENCES route_versions(id) ON DELETE RESTRICT,
    station_id              INTEGER NOT NULL REFERENCES stations(id) ON DELETE RESTRICT,
    stop_sequence           INTEGER NOT NULL,
    distance_from_origin    DECIMAL(10, 2) NOT NULL,
    UNIQUE (route_version_id, stop_sequence),
    UNIQUE (route_version_id, station_id)
);

-- ---------- Trips ----------
-- A single scheduled departure on a given route version.
CREATE TABLE trips (
    id                  SERIAL PRIMARY KEY,
    route_version_id    INTEGER NOT NULL REFERENCES route_versions(id) ON DELETE RESTRICT,
    departure_date      DATE NOT NULL,
    departure_time      TIME NOT NULL,
    status              trip_status NOT NULL DEFAULT 'SCHEDULED',
    UNIQUE (route_version_id, departure_date, departure_time)
);

-- ---------- Coaches ----------
-- Physical fleet inventory.
CREATE TABLE coaches (
    id              SERIAL PRIMARY KEY,
    coach_name      VARCHAR(100) NOT NULL,
    is_reservable   BOOLEAN NOT NULL,
    capacity        INTEGER NOT NULL
);

-- ---------- Trip Coaches ----------
-- Which coaches are attached to a given trip (composition varies per trip).
CREATE TABLE trip_coaches (
    id          SERIAL PRIMARY KEY,
    trip_id     INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    coach_id    INTEGER NOT NULL REFERENCES coaches(id) ON DELETE RESTRICT,
    position    INTEGER NOT NULL,
    UNIQUE (trip_id, coach_id)
);

-- ---------- Seats ----------
-- Physical seats within a coach.
CREATE TABLE seats (
    id              SERIAL PRIMARY KEY,
    coach_id        INTEGER NOT NULL REFERENCES coaches(id) ON DELETE RESTRICT,
    seat_number     VARCHAR(10) NOT NULL,
    UNIQUE (coach_id, seat_number)
);

-- ---------- Trip Seats ----------
-- A seat as it exists on a specific trip. Only populated for seats
-- whose coach is actually attached to that trip via trip_coaches.
CREATE TABLE trip_seats (
    id          SERIAL PRIMARY KEY,
    trip_id     INTEGER NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
    seat_id     INTEGER NOT NULL REFERENCES seats(id) ON DELETE RESTRICT,
    UNIQUE (trip_id, seat_id)
);

-- ---------- Passengers ----------
CREATE TABLE passengers (
    id      SERIAL PRIMARY KEY,
    name    VARCHAR(255) NOT NULL,
    email   VARCHAR(255),
    phone   VARCHAR(50)
);

-- ---------- Bookings ----------
-- A reserved-seat booking for one journey segment.
CREATE TABLE bookings (
    id                  SERIAL PRIMARY KEY,
    passenger_id        INTEGER NOT NULL REFERENCES passengers(id) ON DELETE RESTRICT,
    trip_seat_id        INTEGER NOT NULL REFERENCES trip_seats(id) ON DELETE RESTRICT,
    start_station_id    INTEGER NOT NULL REFERENCES stations(id) ON DELETE RESTRICT,
    end_station_id      INTEGER NOT NULL REFERENCES stations(id) ON DELETE RESTRICT,
    start_sequence      INTEGER NOT NULL,
    end_sequence        INTEGER NOT NULL,
    fare                DECIMAL(10, 2) NOT NULL,
    status              booking_status NOT NULL DEFAULT 'CONFIRMED',
    booking_reference   VARCHAR(20) NOT NULL UNIQUE,
    booking_timestamp   TIMESTAMP NOT NULL DEFAULT now(),

    CHECK (start_sequence < end_sequence),

    -- materialized half-open range, used by the exclusion constraint below
    seg int4range GENERATED ALWAYS AS (int4range(start_sequence, end_sequence, '[)')) STORED
);

-- Prevents two confirmed bookings on the same trip_seat from
-- covering overlapping ranges. Deferrable so it can be checked
-- once at commit time during multi-row updates (e.g. route migration).
ALTER TABLE bookings
    ADD CONSTRAINT no_overlapping_segments
    EXCLUDE USING gist (trip_seat_id WITH =, seg WITH &&)
    WHERE (status = 'CONFIRMED')
    DEFERRABLE INITIALLY IMMEDIATE;

-- ---------- Unreserved Tickets ----------
-- Unreserved (first-come-first-served) ticket sales, no seat assigned.
CREATE TABLE unreserved_tickets (
    id                  SERIAL PRIMARY KEY,
    trip_id             INTEGER NOT NULL REFERENCES trips(id) ON DELETE RESTRICT,
    start_station_id    INTEGER NOT NULL REFERENCES stations(id) ON DELETE RESTRICT,
    end_station_id      INTEGER NOT NULL REFERENCES stations(id) ON DELETE RESTRICT,
    start_sequence      INTEGER NOT NULL,
    end_sequence        INTEGER NOT NULL,
    quantity_sold       INTEGER NOT NULL DEFAULT 1,
    fare                DECIMAL(10, 2) NOT NULL,
    sold_at             TIMESTAMP NOT NULL DEFAULT now(),

    CHECK (start_sequence < end_sequence),
    CHECK (quantity_sold > 0)
);

-- ---------- Helpful indexes ----------
CREATE INDEX idx_bookings_trip_seat ON bookings(trip_seat_id);
CREATE INDEX idx_bookings_passenger ON bookings(passenger_id, status);
CREATE INDEX idx_trip_seats_trip ON trip_seats(trip_id);
CREATE INDEX idx_unreserved_trip ON unreserved_tickets(trip_id);
