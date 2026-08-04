-- name: CreateUnreservedTicket :one
INSERT INTO unreserved_tickets (
    trip_id, start_station_id, end_station_id, start_sequence, end_sequence,
    quantity_sold, fare
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, trip_id, start_station_id, end_station_id, start_sequence,
          end_sequence, quantity_sold, fare, sold_at;
