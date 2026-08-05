-- name: GetTodayRevenue :one
SELECT COALESCE(SUM(fare), 0)::numeric AS revenue, COUNT(*) AS booking_count
FROM bookings
WHERE status = 'CONFIRMED'
  AND confirmed_at::date = CURRENT_DATE;

-- name: ListDailyRevenue :many
SELECT confirmed_at::date AS day, SUM(fare)::numeric AS revenue, COUNT(*) AS booking_count
FROM bookings
WHERE status = 'CONFIRMED'
  AND confirmed_at IS NOT NULL
GROUP BY confirmed_at::date
ORDER BY confirmed_at::date DESC
LIMIT sqlc.arg(day_limit);

-- name: ListRevenueBookingsByDate :many
-- Joins a confirmed booking to the seat, coach, route and trip it was made against, so the revenue log can show what was actually sold, not just a booking ID.
SELECT b.id, b.booking_reference, b.fare, b.confirmed_at,
       p.name AS passenger_name,
       seat.seat_number, coach.coach_name, coach.class,
       t.id AS trip_id, t.departure_date, t.departure_time,
       r.name AS route_name,
       start_st.name AS start_station_name, end_st.name AS end_station_name
FROM bookings b
JOIN passengers p ON p.id = b.passenger_id
JOIN trip_seats ts ON ts.id = b.trip_seat_id
JOIN seats seat ON seat.id = ts.seat_id
JOIN coaches coach ON coach.id = seat.coach_id
JOIN trips t ON t.id = ts.trip_id
JOIN route_versions rv ON rv.id = t.route_version_id
JOIN routes r ON r.id = rv.route_id
JOIN stations start_st ON start_st.id = b.start_station_id
JOIN stations end_st ON end_st.id = b.end_station_id
WHERE b.status = 'CONFIRMED'
  AND b.confirmed_at::date = sqlc.arg(day)
ORDER BY b.confirmed_at DESC
LIMIT sqlc.arg(row_limit) OFFSET sqlc.arg(row_offset);

-- name: CountRevenueBookingsByDate :one
SELECT COUNT(*)
FROM bookings
WHERE status = 'CONFIRMED'
  AND confirmed_at::date = sqlc.arg(day);
