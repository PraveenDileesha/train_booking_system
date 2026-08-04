-- name: CreatePassenger :one
INSERT INTO passengers (name, email, phone)
VALUES ($1, $2, $3)
RETURNING id, name, email, phone;
