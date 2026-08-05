package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PraveenDileesha/train_booking_system/internal/apierror"
	"github.com/PraveenDileesha/train_booking_system/internal/generated"
)

const holdDuration = 5 * time.Minute

// maxSeatsPerBooking caps how many seats one customer can hold in a single request.
// Enforced here, not just in the UI, since the API is the actual trust boundary.
const maxSeatsPerBooking = 5

// BookingHandler serves the reserved-seat booking and seat-availability endpoints.
type BookingHandler struct {
	Pool    *pgxpool.Pool
	Queries *generated.Queries
}

func roundToNearest5(x float64) float64 {
	return math.Round(x/5) * 5
}

func generateBookingReference() (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "BK" + hex.EncodeToString(buf), nil
}

// routeVersionStop is a station's position within a route version, used to compute both the booking's sequence range and its distance-based fare.
type routeVersionStop struct {
	StopSequence       int32
	DistanceFromOrigin float64
}

// stopsByStation is shared by any handler that needs to turn a station pair into a sequence range and distance for a given route version. Booking a reserved seat and issuing an unreserved ticket both need it.
func stopsByStation(ctx context.Context, q *generated.Queries, routeVersionID int32) (map[int32]routeVersionStop, error) {
	rows, err := q.GetRouteVersionStations(ctx, routeVersionID)
	if err != nil {
		return nil, err
	}
	out := make(map[int32]routeVersionStop, len(rows))
	for _, s := range rows {
		dist, err := numericToFloat64(s.DistanceFromOrigin)
		if err != nil {
			return nil, err
		}
		out[s.StationID] = routeVersionStop{StopSequence: s.StopSequence, DistanceFromOrigin: dist}
	}
	return out, nil
}

// ListAvailableSeats returns the seat map for a trip's reserved coaches, each annotated with whether it's free for the requested leg.
func (h *BookingHandler) ListAvailableSeats(w http.ResponseWriter, r *http.Request) {
	tripID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid trip id", http.StatusBadRequest)
		return
	}
	startStationID, err := strconv.Atoi(r.URL.Query().Get("start_station_id"))
	if err != nil {
		http.Error(w, "start_station_id query param is required", http.StatusBadRequest)
		return
	}
	endStationID, err := strconv.Atoi(r.URL.Query().Get("end_station_id"))
	if err != nil {
		http.Error(w, "end_station_id query param is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	trip, err := h.Queries.GetTrip(ctx, int32(tripID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "trip not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to load trip", http.StatusInternalServerError)
		return
	}
	if onlineBookingClosed(combineDateTime(trip.DepartureDate, trip.DepartureTime)) {
		http.Error(w, "online booking for this trip has closed", http.StatusConflict)
		return
	}

	stops, err := stopsByStation(ctx, h.Queries, trip.RouteVersionID)
	if err != nil {
		log.Printf("ListAvailableSeats stopsByStation error: %v (%T)", err, err)
		http.Error(w, "failed to load route stations", http.StatusInternalServerError)
		return
	}

	start, ok := stops[int32(startStationID)]
	if !ok {
		http.Error(w, "start station is not on this trip's route", http.StatusBadRequest)
		return
	}
	end, ok := stops[int32(endStationID)]
	if !ok {
		http.Error(w, "end station is not on this trip's route", http.StatusBadRequest)
		return
	}
	if start.StopSequence >= end.StopSequence {
		http.Error(w, "start station must come before end station on the route", http.StatusBadRequest)
		return
	}

	rows, err := h.Queries.ListTripSeatsWithAvailability(ctx, generated.ListTripSeatsWithAvailabilityParams{
		StartSequence: start.StopSequence,
		EndSequence:   end.StopSequence,
		TripID:        int32(tripID),
	})
	if err != nil {
		log.Printf("ListAvailableSeats error: %v (%T)", err, err)
		http.Error(w, "failed to list seats", http.StatusInternalServerError)
		return
	}

	// Only reserved coaches have individually bookable seats. Unreserved coaches are first-come-first-served and don't appear on the seat map.
	seats := make([]generated.ListTripSeatsWithAvailabilityRow, 0, len(rows))
	for _, s := range rows {
		if s.IsReservable {
			seats = append(seats, s)
		}
	}

	// Fares come along with the seat map so the frontend can price every seat without a second request just to fetch trip detail.
	fares, err := h.Queries.ListTripFares(ctx, int32(tripID))
	if err != nil {
		log.Printf("ListAvailableSeats ListTripFares error: %v (%T)", err, err)
		http.Error(w, "failed to load fares", http.StatusInternalServerError)
		return
	}

	distanceKm := end.DistanceFromOrigin - start.DistanceFromOrigin

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"seats":       seats,
		"fares":       fares,
		"distance_km": distanceKm,
	})
}

type bookingPassengerInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
}

type createBookingRequest struct {
	TripSeatIDs    []int32               `json:"trip_seat_ids"`
	StartStationID int32                 `json:"start_station_id"`
	EndStationID   int32                 `json:"end_station_id"`
	Passenger      bookingPassengerInput `json:"passenger"`
}

type bookingResponse struct {
	ID               int32   `json:"id"`
	TripSeatID       int32   `json:"trip_seat_id"`
	StartStationID   int32   `json:"start_station_id"`
	EndStationID     int32   `json:"end_station_id"`
	Fare             float64 `json:"fare"`
	Status           string  `json:"status"`
	BookingReference string  `json:"booking_reference"`
	HeldUntil        *string `json:"held_until,omitempty"`
}

// seatBookingError is a client-facing failure tied to one seat within a bulk booking request.
// It carries the HTTP status the whole request should fail with, since one bad seat fails the entire (single-transaction) hold.
type seatBookingError struct {
	status int
	msg    string
}

func (e *seatBookingError) Error() string { return e.msg }

// bookOneSeat runs the full hold pipeline (seat lookup, route and fare validation, stale-hold cleanup, booking insert) for a single seat within an already-open transaction, so CreateBooking can call it once per seat and hold every seat in the request atomically.
func bookOneSeat(ctx context.Context, qtx *generated.Queries, tripSeatID, startStationID, endStationID, passengerID int32) (bookingResponse, error) {
	tripSeat, err := qtx.GetTripSeat(ctx, tripSeatID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingResponse{}, &seatBookingError{http.StatusNotFound, fmt.Sprintf("seat %d not found on this trip", tripSeatID)}
		}
		return bookingResponse{}, fmt.Errorf("failed to load seat: %w", err)
	}
	if !tripSeat.IsReservable {
		return bookingResponse{}, &seatBookingError{http.StatusBadRequest, fmt.Sprintf("seat %d is in an unreserved coach and cannot be individually booked", tripSeatID)}
	}
	if onlineBookingClosed(combineDateTime(tripSeat.DepartureDate, tripSeat.DepartureTime)) {
		return bookingResponse{}, &seatBookingError{http.StatusConflict, fmt.Sprintf("online booking for seat %d's trip has closed", tripSeatID)}
	}

	stops, err := stopsByStation(ctx, qtx, tripSeat.RouteVersionID)
	if err != nil {
		return bookingResponse{}, fmt.Errorf("failed to load route stations: %w", err)
	}
	start, ok := stops[startStationID]
	if !ok {
		return bookingResponse{}, &seatBookingError{http.StatusBadRequest, "start station is not on this trip's route"}
	}
	end, ok := stops[endStationID]
	if !ok {
		return bookingResponse{}, &seatBookingError{http.StatusBadRequest, "end station is not on this trip's route"}
	}
	if start.StopSequence >= end.StopSequence {
		return bookingResponse{}, &seatBookingError{http.StatusBadRequest, "start station must come before end station on the route"}
	}

	fareRow, err := qtx.GetTripFare(ctx, generated.GetTripFareParams{
		TripID:       tripSeat.TripID,
		Class:        tripSeat.Class,
		IsReservable: true,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return bookingResponse{}, &seatBookingError{http.StatusConflict, "no fare configured for this seat's class on this trip"}
		}
		return bookingResponse{}, fmt.Errorf("failed to load fare: %w", err)
	}
	ratePerKm, err := numericToFloat64(fareRow.RatePerKm)
	if err != nil {
		return bookingResponse{}, fmt.Errorf("invalid fare rate: %w", err)
	}
	distanceKm := end.DistanceFromOrigin - start.DistanceFromOrigin
	fare := roundToNearest5(distanceKm * ratePerKm)
	fareNumeric, err := numericFromFloat64(fare)
	if err != nil {
		return bookingResponse{}, fmt.Errorf("failed to compute fare: %w", err)
	}

	reference, err := generateBookingReference()
	if err != nil {
		return bookingResponse{}, fmt.Errorf("failed to generate booking reference: %w", err)
	}

	// Clear out any abandoned hold on this seat first, so it doesn't spuriously block this attempt via the exclusion constraint.
	if err := qtx.ExpireStaleHoldsForTripSeat(ctx, tripSeat.ID); err != nil {
		return bookingResponse{}, fmt.Errorf("failed to check seat availability: %w", err)
	}

	heldUntil := pgtype.Timestamp{Time: time.Now().Add(holdDuration), Valid: true}

	booking, err := qtx.CreateBooking(ctx, generated.CreateBookingParams{
		PassengerID:      passengerID,
		TripSeatID:       tripSeat.ID,
		StartStationID:   startStationID,
		EndStationID:     endStationID,
		StartSequence:    start.StopSequence,
		EndSequence:      end.StopSequence,
		Fare:             fareNumeric,
		BookingReference: reference,
		HeldUntil:        heldUntil,
	})
	if err != nil {
		return bookingResponse{}, err
	}

	held := booking.HeldUntil.Time.Format(time.RFC3339)
	return bookingResponse{
		ID:               booking.ID,
		TripSeatID:       booking.TripSeatID,
		StartStationID:   booking.StartStationID,
		EndStationID:     booking.EndStationID,
		Fare:             fare,
		Status:           string(booking.Status),
		BookingReference: booking.BookingReference,
		HeldUntil:        &held,
	}, nil
}

// CreateBooking places a 5-minute hold on up to maxSeatsPerBooking seats for one passenger on a specific leg, all in a single transaction.
// Each hold is blocked from overlapping with any other PENDING or CONFIRMED booking on that seat by the no_overlapping_segments exclusion constraint, so a race between two concurrent requests for the same range is resolved atomically by Postgres, not by application-level locking.
func (h *BookingHandler) CreateBooking(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Passenger.Name == "" {
		http.Error(w, "passenger name is required", http.StatusBadRequest)
		return
	}
	if len(req.TripSeatIDs) == 0 {
		http.Error(w, "at least one seat must be selected", http.StatusBadRequest)
		return
	}
	if len(req.TripSeatIDs) > maxSeatsPerBooking {
		http.Error(w, fmt.Sprintf("cannot book more than %d seats at a time", maxSeatsPerBooking), http.StatusBadRequest)
		return
	}
	seen := make(map[int32]bool, len(req.TripSeatIDs))
	for _, id := range req.TripSeatIDs {
		if seen[id] {
			http.Error(w, "the same seat was selected more than once", http.StatusBadRequest)
			return
		}
		seen[id] = true
	}

	ctx := r.Context()

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)

	var email, phone *string
	if req.Passenger.Email != "" {
		email = &req.Passenger.Email
	}
	if req.Passenger.Phone != "" {
		phone = &req.Passenger.Phone
	}
	passenger, err := qtx.CreatePassenger(ctx, generated.CreatePassengerParams{
		Name:  req.Passenger.Name,
		Email: email,
		Phone: phone,
	})
	if err != nil {
		log.Printf("CreateBooking CreatePassenger error: %v (%T)", err, err)
		http.Error(w, "failed to save passenger", http.StatusInternalServerError)
		return
	}

	bookings := make([]bookingResponse, 0, len(req.TripSeatIDs))
	for _, tripSeatID := range req.TripSeatIDs {
		resp, err := bookOneSeat(ctx, qtx, tripSeatID, req.StartStationID, req.EndStationID, passenger.ID)
		if err != nil {
			var seatErr *seatBookingError
			if errors.As(err, &seatErr) {
				http.Error(w, seatErr.msg, seatErr.status)
				return
			}
			if apierror.WritePostgresError(w, err) {
				return
			}
			log.Printf("CreateBooking bookOneSeat error: %v (%T)", err, err)
			http.Error(w, "failed to create booking", http.StatusInternalServerError)
			return
		}
		bookings = append(bookings, resp)
	}

	if err := tx.Commit(ctx); err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		http.Error(w, "failed to save booking", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"bookings": bookings})
}

// ConfirmBooking finalizes a hold.
// It fails if the hold already expired (the customer took longer than 5 minutes).
// The seat may since have been taken by someone else, so the client should re-search.
func (h *BookingHandler) ConfirmBooking(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid booking id", http.StatusBadRequest)
		return
	}

	booking, err := h.Queries.ConfirmBooking(r.Context(), int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "booking hold has expired or was already confirmed or cancelled", http.StatusConflict)
			return
		}
		log.Printf("ConfirmBooking error: %v (%T)", err, err)
		http.Error(w, "failed to confirm booking", http.StatusInternalServerError)
		return
	}

	fare, err := numericToFloat64(booking.Fare)
	if err != nil {
		http.Error(w, "invalid fare on booking", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bookingResponse{
		ID:               booking.ID,
		TripSeatID:       booking.TripSeatID,
		StartStationID:   booking.StartStationID,
		EndStationID:     booking.EndStationID,
		Fare:             fare,
		Status:           string(booking.Status),
		BookingReference: booking.BookingReference,
	})
}

// CancelBooking releases a hold before it naturally expires or cancels an already-confirmed booking.
// The seat becomes bookable by someone else immediately, rather than waiting out held_until.
func (h *BookingHandler) CancelBooking(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid booking id", http.StatusBadRequest)
		return
	}

	rowsAffected, err := h.Queries.CancelBooking(r.Context(), int32(id))
	if err != nil {
		log.Printf("CancelBooking error: %v (%T)", err, err)
		http.Error(w, "failed to cancel booking", http.StatusInternalServerError)
		return
	}
	if rowsAffected == 0 {
		http.Error(w, "booking not found or already cancelled or expired", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetBooking returns a single booking by ID.
func (h *BookingHandler) GetBooking(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid booking id", http.StatusBadRequest)
		return
	}

	booking, err := h.Queries.GetBooking(r.Context(), int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "booking not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get booking", http.StatusInternalServerError)
		return
	}

	fare, err := numericToFloat64(booking.Fare)
	if err != nil {
		http.Error(w, "invalid fare on booking", http.StatusInternalServerError)
		return
	}

	var held *string
	if booking.HeldUntil.Valid {
		s := booking.HeldUntil.Time.Format(time.RFC3339)
		held = &s
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bookingResponse{
		ID:               booking.ID,
		TripSeatID:       booking.TripSeatID,
		StartStationID:   booking.StartStationID,
		EndStationID:     booking.EndStationID,
		Fare:             fare,
		Status:           string(booking.Status),
		BookingReference: booking.BookingReference,
		HeldUntil:        held,
	})
}
