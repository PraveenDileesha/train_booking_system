package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PraveenDileesha/train_booking_system/internal/apierror"
	"github.com/PraveenDileesha/train_booking_system/internal/generated"
)

type TripHandler struct {
	Pool    *pgxpool.Pool
	Queries *generated.Queries
}

func dateFromString(s string) (pgtype.Date, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return pgtype.Date{}, err
	}
	return pgtype.Date{Time: t, Valid: true}, nil
}

func dateToString(d pgtype.Date) string {
	return d.Time.Format("2006-01-02")
}

func timeFromString(s string) (pgtype.Time, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return pgtype.Time{}, err
	}
	micros := int64(t.Hour())*3600e6 + int64(t.Minute())*60e6
	return pgtype.Time{Microseconds: micros, Valid: true}, nil
}

func timeToString(t pgtype.Time) string {
	total := t.Microseconds / 1e6
	h := total / 3600
	m := (total % 3600) / 60
	return pad2(int(h)) + ":" + pad2(int(m))
}

func pad2(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}
	return strconv.Itoa(n)
}

type tripResponse struct {
	ID             int32  `json:"id"`
	RouteVersionID int32  `json:"route_version_id"`
	DepartureDate  string `json:"departure_date"`
	DepartureTime  string `json:"departure_time"`
	ArrivalDate    string `json:"arrival_date"`
	ArrivalTime    string `json:"arrival_time"`
	Status         string `json:"status"`
	RouteName      string `json:"route_name,omitempty"`
}

func toTripResponse(t generated.Trip) tripResponse {
	return tripResponse{
		ID:             t.ID,
		RouteVersionID: t.RouteVersionID,
		DepartureDate:  dateToString(t.DepartureDate),
		DepartureTime:  timeToString(t.DepartureTime),
		ArrivalDate:    dateToString(t.ArrivalDate),
		ArrivalTime:    timeToString(t.ArrivalTime),
		Status:         string(t.Status),
	}
}

type createTripFareInput struct {
	Class        generated.CoachClass `json:"class"`
	IsReservable bool                 `json:"is_reservable"`
	RatePerKm    float64              `json:"rate_per_km"`
}

type createTripRequest struct {
	RouteID       int32                 `json:"route_id"`
	DepartureDate string                `json:"departure_date"`
	DepartureTime string                `json:"departure_time"`
	ArrivalDate   string                `json:"arrival_date"`
	ArrivalTime   string                `json:"arrival_time"`
	CoachIDs      []int32               `json:"coach_ids"`
	Fares         []createTripFareInput `json:"fares"`
}

// CreateTrip schedules a trip on the route's currently active version, attaches the admin-chosen coaches (populating trip_seats for each of their seats), and records the per-class fare rates for that trip.
func (h *TripHandler) CreateTrip(w http.ResponseWriter, r *http.Request) {
	var req createTripRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.CoachIDs) == 0 {
		http.Error(w, "at least one coach must be attached to the trip", http.StatusBadRequest)
		return
	}
	if len(req.Fares) == 0 {
		http.Error(w, "at least one fare rate is required", http.StatusBadRequest)
		return
	}

	date, err := dateFromString(req.DepartureDate)
	if err != nil {
		http.Error(w, "invalid departure_date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	depTime, err := timeFromString(req.DepartureTime)
	if err != nil {
		http.Error(w, "invalid departure_time, expected HH:MM", http.StatusBadRequest)
		return
	}
	arrDate, err := dateFromString(req.ArrivalDate)
	if err != nil {
		http.Error(w, "invalid arrival_date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	arrTime, err := timeFromString(req.ArrivalTime)
	if err != nil {
		http.Error(w, "invalid arrival_time, expected HH:MM", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	activeVersion, err := h.Queries.GetActiveRouteVersion(ctx, req.RouteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "route not found or has no active version", http.StatusNotFound)
			return
		}
		log.Printf("CreateTrip GetActiveRouteVersion error: %v (%T)", err, err)
		http.Error(w, "failed to load route version", http.StatusInternalServerError)
		return
	}

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)

	trip, err := qtx.CreateTrip(ctx, generated.CreateTripParams{
		RouteVersionID: activeVersion.ID,
		DepartureDate:  date,
		DepartureTime:  depTime,
		ArrivalDate:    arrDate,
		ArrivalTime:    arrTime,
		Status:         generated.TripStatusSCHEDULED,
	})
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("CreateTrip CreateTrip error: %v (%T)", err, err)
		http.Error(w, "failed to create trip", http.StatusInternalServerError)
		return
	}

	if err := qtx.CreateTripCoaches(ctx, generated.CreateTripCoachesParams{
		TripID:   trip.ID,
		CoachIds: req.CoachIDs,
	}); err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("CreateTrip CreateTripCoaches error: %v (%T)", err, err)
		http.Error(w, "failed to attach coaches to trip", http.StatusInternalServerError)
		return
	}

	if err := qtx.CreateTripSeatsForCoaches(ctx, generated.CreateTripSeatsForCoachesParams{
		TripID:   trip.ID,
		CoachIds: req.CoachIDs,
	}); err != nil {
		log.Printf("CreateTrip CreateTripSeatsForCoaches error: %v (%T)", err, err)
		http.Error(w, "failed to make coach seats available on trip", http.StatusInternalServerError)
		return
	}

	for _, fare := range req.Fares {
		if !validCoachClass(fare.Class) {
			http.Error(w, "fare class must be one of FIRST_AC, SECOND, THIRD", http.StatusBadRequest)
			return
		}
		if fare.RatePerKm <= 0 {
			http.Error(w, "rate_per_km must be positive", http.StatusBadRequest)
			return
		}
		rate, err := numericFromFloat64(fare.RatePerKm)
		if err != nil {
			http.Error(w, "invalid rate_per_km value", http.StatusBadRequest)
			return
		}
		if _, err := qtx.CreateTripFare(ctx, generated.CreateTripFareParams{
			TripID:       trip.ID,
			Class:        fare.Class,
			IsReservable: fare.IsReservable,
			RatePerKm:    rate,
		}); err != nil {
			if apierror.WritePostgresError(w, err) {
				return
			}
			log.Printf("CreateTrip CreateTripFare error: %v (%T)", err, err)
			http.Error(w, "failed to save fare", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to save trip", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(toTripResponse(trip))
}

// ListTrips returns a paginated list of trips.
func (h *TripHandler) ListTrips(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)

	rows, err := h.Queries.ListTrips(r.Context(), generated.ListTripsParams{
		RowLimit:  int32(pageSize),
		RowOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		log.Printf("ListTrips error: %v (%T)", err, err)
		http.Error(w, "failed to list trips", http.StatusInternalServerError)
		return
	}

	total, err := h.Queries.CountTrips(r.Context())
	if err != nil {
		log.Printf("CountTrips error: %v (%T)", err, err)
		http.Error(w, "failed to list trips", http.StatusInternalServerError)
		return
	}

	resp := make([]map[string]any, 0, len(rows))
	for _, t := range rows {
		resp = append(resp, map[string]any{
			"id":               t.ID,
			"route_version_id": t.RouteVersionID,
			"departure_date":   dateToString(t.DepartureDate),
			"departure_time":   timeToString(t.DepartureTime),
			"arrival_date":     dateToString(t.ArrivalDate),
			"arrival_time":     timeToString(t.ArrivalTime),
			"status":           t.Status,
			"route_id":         t.RouteID,
			"route_name":       t.RouteName,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(resp, page, pageSize, total))
}

// DeleteTrip removes a trip that has no activity against it.
// Fails with a 409 if any booking or unreserved ticket references it: bookings RESTRICT trip_seats, and unreserved_tickets RESTRICTs trips directly.
func (h *TripHandler) DeleteTrip(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid trip id", http.StatusBadRequest)
		return
	}

	rowsAffected, err := h.Queries.DeleteTrip(r.Context(), int32(id))
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("DeleteTrip unmapped error: %v (%T)", err, err)
		http.Error(w, "failed to delete trip", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "trip not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTrip returns full detail for a trip: its route's stations (with sequence and distance, needed by the frontend to compute legs), attached coaches, and fare rates.
// Used by both the admin trip page and the customer booking flow.
func (h *TripHandler) GetTrip(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid trip id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	trip, err := h.Queries.GetTrip(ctx, int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "trip not found", http.StatusNotFound)
			return
		}
		log.Printf("GetTrip error: %v (%T)", err, err)
		http.Error(w, "failed to get trip", http.StatusInternalServerError)
		return
	}

	stations, err := h.Queries.GetRouteVersionStations(ctx, trip.RouteVersionID)
	if err != nil {
		log.Printf("GetTrip GetRouteVersionStations error: %v (%T)", err, err)
		http.Error(w, "failed to load route stations", http.StatusInternalServerError)
		return
	}

	coaches, err := h.Queries.ListTripCoaches(ctx, trip.ID)
	if err != nil {
		log.Printf("GetTrip ListTripCoaches error: %v (%T)", err, err)
		http.Error(w, "failed to load trip coaches", http.StatusInternalServerError)
		return
	}

	fares, err := h.Queries.ListTripFares(ctx, trip.ID)
	if err != nil {
		log.Printf("GetTrip ListTripFares error: %v (%T)", err, err)
		http.Error(w, "failed to load trip fares", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"trip":     toTripResponse(trip),
		"stations": stations,
		"coaches":  coaches,
		"fares":    fares,
	})
}

// SearchTrips finds trips departing on the given date whose route passes through both stations in that order.
// Matches by station pair rather than by route, since customers search by where they are going, not by which route serves them.
func (h *TripHandler) SearchTrips(w http.ResponseWriter, r *http.Request) {
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
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		http.Error(w, "date query param is required (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	date, err := dateFromString(dateStr)
	if err != nil {
		http.Error(w, "invalid date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}

	page, pageSize := parsePagination(r)

	trips, err := h.Queries.SearchTrips(r.Context(), generated.SearchTripsParams{
		StartStationID: int32(startStationID),
		EndStationID:   int32(endStationID),
		DepartureDate:  date,
		RowLimit:       int32(pageSize),
		RowOffset:      int32((page - 1) * pageSize),
	})
	if err != nil {
		log.Printf("SearchTrips error: %v (%T)", err, err)
		http.Error(w, "failed to search trips", http.StatusInternalServerError)
		return
	}

	total, err := h.Queries.CountSearchTrips(r.Context(), generated.CountSearchTripsParams{
		StartStationID: int32(startStationID),
		EndStationID:   int32(endStationID),
		DepartureDate:  date,
	})
	if err != nil {
		log.Printf("CountSearchTrips error: %v (%T)", err, err)
		http.Error(w, "failed to search trips", http.StatusInternalServerError)
		return
	}

	resp := make([]tripResponse, 0, len(trips))
	for _, t := range trips {
		tr := toTripResponse(generated.Trip{
			ID:             t.ID,
			RouteVersionID: t.RouteVersionID,
			DepartureDate:  t.DepartureDate,
			DepartureTime:  t.DepartureTime,
			ArrivalDate:    t.ArrivalDate,
			ArrivalTime:    t.ArrivalTime,
			Status:         t.Status,
		})
		tr.RouteName = t.RouteName
		resp = append(resp, tr)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(resp, page, pageSize, total))
}
