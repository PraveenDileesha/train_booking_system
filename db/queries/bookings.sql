-- name: ExpireStaleHoldsForTripSeat :exec
UPDATE bookings
SET status = 'EXPIRED'
WHERE trip_seat_id = $1
  AND status = 'PENDING'
  AND held_until < now();

-- name: CreateBooking :one
INSERT INTO bookings (
    passenger_id, trip_seat_id, start_station_id, end_station_id,
    start_sequence, end_sequence, fare, status, booking_reference, held_until
)
VALUES ($1, $2, $3, $4, $5, $6, $7, 'PENDING', $8, $9)
RETURNING id, passenger_id, trip_seat_id, start_station_id, end_station_id,
          start_sequence, end_sequence, fare, status, booking_reference,
          booking_timestamp, held_until;

-- name: ConfirmBooking :one
UPDATE bookings
SET status = 'CONFIRMED', held_until = NULL
WHERE id = $1
  AND status = 'PENDING'
  AND held_until >= now()
RETURNING id, passenger_id, trip_seat_id, start_station_id, end_station_id,
          start_sequence, end_sequence, fare, status, booking_reference,
          booking_timestamp, held_until;

-- name: GetBooking :one
SELECT id, passenger_id, trip_seat_id, start_station_id, end_station_id,
       start_sequence, end_sequence, fare, status, booking_reference,
       booking_timestamp, held_until
FROM bookings
WHERE id = $1;

-- name: CancelBooking :execrows
UPDATE bookings
SET status = 'CANCELLED', held_until = NULL
WHERE id = $1
  AND status IN ('PENDING', 'CONFIRMED');
