package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PraveenDileesha/train_booking_system/internal/apierror"
	"github.com/PraveenDileesha/train_booking_system/internal/generated"
)

// CounterHandler serves the unreserved ticket-counter sale endpoint.
type CounterHandler struct {
	Pool    *pgxpool.Pool
	Queries *generated.Queries
}

// maxTicketsPerIssue caps how many unreserved tickets one counter transaction can sell at once.
// Mirrors the reserved-seat booking limit in bookings.go, so a single sale can't silently sweep up an unbounded block of tickets.
const maxTicketsPerIssue = 5

type issueUnreservedTicketRequest struct {
	TripID         int32                `json:"trip_id"`
	StartStationID int32                `json:"start_station_id"`
	EndStationID   int32                `json:"end_station_id"`
	Class          generated.CoachClass `json:"class"`
	Quantity       int32                `json:"quantity"`
}

type unreservedTicketResponse struct {
	ID             int32   `json:"id"`
	TripID         int32   `json:"trip_id"`
	StartStationID int32   `json:"start_station_id"`
	EndStationID   int32   `json:"end_station_id"`
	Class          string  `json:"class"`
	Quantity       int32   `json:"quantity"`
	FarePerTicket  float64 `json:"fare_per_ticket"`
	TotalFare      float64 `json:"total_fare"`
}

// IssueUnreservedTicket records a first-come-first-served ticket sale.
// There is no seat, no hold and no coach-capacity check.
// Unreserved coaches have no per-seat inventory, matching the real system this replaces.
// maxTicketsPerIssue still caps a single sale.
// The row exists purely for revenue tracking.
func (h *CounterHandler) IssueUnreservedTicket(w http.ResponseWriter, r *http.Request) {
	var req issueUnreservedTicketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if !validCoachClass(req.Class) {
		http.Error(w, "class must be one of FIRST_AC, SECOND, THIRD", http.StatusBadRequest)
		return
	}
	if req.Quantity < 1 {
		http.Error(w, "quantity must be at least 1", http.StatusBadRequest)
		return
	}
	if req.Quantity > maxTicketsPerIssue {
		http.Error(w, fmt.Sprintf("cannot issue more than %d tickets at a time", maxTicketsPerIssue), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	trip, err := h.Queries.GetTrip(ctx, req.TripID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "trip not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load trip", http.StatusInternalServerError)
		return
	}
	if hasDeparted(trip.Status, combineDateTime(trip.DepartureDate, trip.DepartureTime), combineDateTime(trip.ArrivalDate, trip.ArrivalTime)) {
		http.Error(w, "this trip has already departed", http.StatusConflict)
		return
	}

	stops, err := stopsByStation(ctx, h.Queries, trip.RouteVersionID)
	if err != nil {
		log.Printf("IssueUnreservedTicket stopsByStation error: %v (%T)", err, err)
		http.Error(w, "failed to load route stations", http.StatusInternalServerError)
		return
	}
	start, ok := stops[req.StartStationID]
	if !ok {
		http.Error(w, "start station is not on this trip's route", http.StatusBadRequest)
		return
	}
	end, ok := stops[req.EndStationID]
	if !ok {
		http.Error(w, "end station is not on this trip's route", http.StatusBadRequest)
		return
	}
	if start.StopSequence >= end.StopSequence {
		http.Error(w, "start station must come before end station on the route", http.StatusBadRequest)
		return
	}

	fareRow, err := h.Queries.GetTripFare(ctx, generated.GetTripFareParams{
		TripID:       trip.ID,
		Class:        req.Class,
		IsReservable: false,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "no unreserved fare configured for this class on this trip", http.StatusConflict)
			return
		}
		http.Error(w, "failed to load fare", http.StatusInternalServerError)
		return
	}
	ratePerKm, err := numericToFloat64(fareRow.RatePerKm)
	if err != nil {
		http.Error(w, "invalid fare rate", http.StatusInternalServerError)
		return
	}
	distanceKm := end.DistanceFromOrigin - start.DistanceFromOrigin
	farePerTicket := roundToNearest5(distanceKm * ratePerKm)
	totalFare := farePerTicket * float64(req.Quantity)

	totalFareNumeric, err := numericFromFloat64(totalFare)
	if err != nil {
		http.Error(w, "failed to compute fare", http.StatusInternalServerError)
		return
	}

	ticket, err := h.Queries.CreateUnreservedTicket(ctx, generated.CreateUnreservedTicketParams{
		TripID:         trip.ID,
		StartStationID: req.StartStationID,
		EndStationID:   req.EndStationID,
		StartSequence:  start.StopSequence,
		EndSequence:    end.StopSequence,
		QuantitySold:   req.Quantity,
		Fare:           totalFareNumeric,
	})
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("IssueUnreservedTicket CreateUnreservedTicket error: %v (%T)", err, err)
		http.Error(w, "failed to issue ticket", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(unreservedTicketResponse{
		ID:             ticket.ID,
		TripID:         ticket.TripID,
		StartStationID: ticket.StartStationID,
		EndStationID:   ticket.EndStationID,
		Class:          string(req.Class),
		Quantity:       ticket.QuantitySold,
		FarePerTicket:  farePerTicket,
		TotalFare:      totalFare,
	})
}
