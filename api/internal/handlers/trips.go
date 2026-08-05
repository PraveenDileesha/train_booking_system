package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
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

// TripHandler serves trip scheduling, search and detail endpoints.
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

// combineDateTime merges a separately-stored date and time of day into one instant.
func combineDateTime(d pgtype.Date, t pgtype.Time) time.Time {
	return d.Time.Add(time.Duration(t.Microseconds) * time.Microsecond)
}

// nextInstant combines hhmm with the same calendar day as after, rolling to the next day if that would land before after.
// Used to infer each stop's date along a route from nothing but a time of day, in stop order.
func nextInstant(after time.Time, hhmm string) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	candidate := time.Date(after.Year(), after.Month(), after.Day(), t.Hour(), t.Minute(), 0, 0, after.Location())
	if candidate.Before(after) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate, nil
}

// boardingWindow is how long before departure a trip shows as BOARDING rather than SCHEDULED.
const boardingWindow = 15 * time.Minute

// effectiveTripStatus computes a trip's current status from its departure and arrival instants, rather than from the stored column, which only ever holds SCHEDULED or CANCELLED.
// CANCELLED is a terminal state an admin sets directly and is never overridden here.
func effectiveTripStatus(stored generated.TripStatus, departure, arrival time.Time) generated.TripStatus {
	if stored == generated.TripStatusCANCELLED {
		return stored
	}
	now := time.Now()
	switch {
	case now.Before(departure.Add(-boardingWindow)):
		return generated.TripStatusSCHEDULED
	case now.Before(departure):
		return generated.TripStatusBOARDING
	case now.Before(arrival):
		return generated.TripStatusDEPARTED
	default:
		return generated.TripStatusCOMPLETED
	}
}

// hasDeparted reports whether a trip is no longer bookable because it has already left, arrived, or been cancelled.
// Used for the ticket counter, where a walk-up passenger can still be sold a ticket right up until the train actually leaves.
func hasDeparted(stored generated.TripStatus, departure, arrival time.Time) bool {
	switch effectiveTripStatus(stored, departure, arrival) {
	case generated.TripStatusDEPARTED, generated.TripStatusCOMPLETED, generated.TripStatusCANCELLED:
		return true
	default:
		return false
	}
}

// onlineBookingCutoff is how long before origin departure online reservations close.
// Applied uniformly regardless of which station a passenger boards at, since only the origin's departure time is tracked, not per-stop arrival times.
const onlineBookingCutoff = 2 * time.Hour

// onlineBookingClosed reports whether reserved-seat booking is closed for a trip, either because departure is less than onlineBookingCutoff away or because it has already departed.
func onlineBookingClosed(departure time.Time) bool {
	return !time.Now().Before(departure.Add(-onlineBookingCutoff))
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

// tripStationResponse is one stop on a trip's route, with the trip's own departure and arrival filling the first and last stop and trip_stop_times filling every stop in between.
type tripStationResponse struct {
	ID                 int32   `json:"id"`
	StationID          int32   `json:"station_id"`
	StationName        string  `json:"station_name"`
	StopSequence       int32   `json:"stop_sequence"`
	DistanceFromOrigin float64 `json:"distance_from_origin"`
	ArrivalTime        *string `json:"arrival_time,omitempty"`
	DepartureTime      *string `json:"departure_time,omitempty"`
}

// searchTripResponse adds the customer's own boarding and alighting times to a trip, since a passenger searching between two stations cares about their own leg, not necessarily the trip's overall origin departure and destination arrival.
type searchTripResponse struct {
	tripResponse
	BoardingArrival   string `json:"boarding_arrival"`
	BoardingDeparture string `json:"boarding_departure"`
	AlightingArrival  string `json:"alighting_arrival"`
}

func toTripResponse(t generated.Trip) tripResponse {
	return tripResponse{
		ID:             t.ID,
		RouteVersionID: t.RouteVersionID,
		DepartureDate:  dateToString(t.DepartureDate),
		DepartureTime:  timeToString(t.DepartureTime),
		ArrivalDate:    dateToString(t.ArrivalDate),
		ArrivalTime:    timeToString(t.ArrivalTime),
		Status:         string(effectiveTripStatus(t.Status, combineDateTime(t.DepartureDate, t.DepartureTime), combineDateTime(t.ArrivalDate, t.ArrivalTime))),
	}
}

type createTripFareInput struct {
	Class        generated.CoachClass `json:"class"`
	IsReservable bool                 `json:"is_reservable"`
	RatePerKm    float64              `json:"rate_per_km"`
}

type createTripStopInput struct {
	RouteStationID int32  `json:"route_station_id"`
	ArrivalTime    string `json:"arrival_time"`
	DepartureTime  string `json:"departure_time"`
}

type createTripRequest struct {
	RouteID       int32                 `json:"route_id"`
	DepartureDate string                `json:"departure_date"`
	DepartureTime string                `json:"departure_time"`
	ArrivalDate   string                `json:"arrival_date"`
	ArrivalTime   string                `json:"arrival_time"`
	CoachIDs      []int32               `json:"coach_ids"`
	Fares         []createTripFareInput `json:"fares"`
	Stops         []createTripStopInput `json:"stops"`
}

// CreateTrip schedules a trip on the route's currently active version, attaches the admin-chosen coaches (populating trip_seats for each of their seats) and records the per-class fare rates for that trip.
// Requires an arrival and departure time for every intermediate station on the route, strictly increasing between the trip's own departure and arrival.
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

	stations, err := h.Queries.GetRouteVersionStations(ctx, activeVersion.ID)
	if err != nil {
		log.Printf("CreateTrip GetRouteVersionStations error: %v (%T)", err, err)
		http.Error(w, "failed to load route stations", http.StatusInternalServerError)
		return
	}
	if len(stations) < 2 {
		http.Error(w, "route has fewer than 2 stations", http.StatusInternalServerError)
		return
	}
	intermediateStops := stations[1 : len(stations)-1]
	if len(req.Stops) != len(intermediateStops) {
		http.Error(w, fmt.Sprintf("expected stop times for %d intermediate station(s), got %d", len(intermediateStops), len(req.Stops)), http.StatusBadRequest)
		return
	}
	stopByRouteStationID := make(map[int32]createTripStopInput, len(req.Stops))
	for _, s := range req.Stops {
		stopByRouteStationID[s.RouteStationID] = s
	}

	departureInstant := combineDateTime(date, depTime)
	arrivalInstant := combineDateTime(arrDate, arrTime)
	cursor := departureInstant
	stopParams := make([]generated.CreateTripStopTimesParams, 0, len(intermediateStops))
	for _, st := range intermediateStops {
		input, ok := stopByRouteStationID[st.ID]
		if !ok {
			http.Error(w, fmt.Sprintf("missing stop time for station %q", st.StationName), http.StatusBadRequest)
			return
		}
		arrival, err := nextInstant(cursor, input.ArrivalTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid arrival_time for station %q, expected HH:MM", st.StationName), http.StatusBadRequest)
			return
		}
		departure, err := nextInstant(arrival, input.DepartureTime)
		if err != nil {
			http.Error(w, fmt.Sprintf("invalid departure_time for station %q, expected HH:MM", st.StationName), http.StatusBadRequest)
			return
		}
		cursor = departure
		stopParams = append(stopParams, generated.CreateTripStopTimesParams{
			RouteStationID: st.ID,
			ArrivalTime:    pgtype.Timestamp{Time: arrival, Valid: true},
			DepartureTime:  pgtype.Timestamp{Time: departure, Valid: true},
		})
	}
	if cursor.After(arrivalInstant) {
		http.Error(w, "stop times exceed the trip's arrival time", http.StatusBadRequest)
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

	if len(stopParams) > 0 {
		for i := range stopParams {
			stopParams[i].TripID = trip.ID
		}
		if _, err := qtx.CreateTripStopTimes(ctx, stopParams); err != nil {
			log.Printf("CreateTrip CreateTripStopTimes error: %v (%T)", err, err)
			http.Error(w, "failed to save stop times", http.StatusInternalServerError)
			return
		}
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

// ListTrips returns a paginated list of trips, each with has_activity so the caller can tell whether DeleteTrip would succeed without having to try it.
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
			"status":           effectiveTripStatus(t.Status, combineDateTime(t.DepartureDate, t.DepartureTime), combineDateTime(t.ArrivalDate, t.ArrivalTime)),
			"route_id":         t.RouteID,
			"route_name":       t.RouteName,
			"has_activity":     t.HasActivity,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(resp, page, pageSize, total))
}

// DeleteTrip removes a trip that has no activity against it.
// Fails with a 409 if any booking or unreserved ticket references it.
// Bookings RESTRICT trip_seats and unreserved_tickets RESTRICTs trips directly.
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

// GetTrip returns full detail for a trip, including its route's stations (with sequence and distance, needed by the frontend to compute legs), attached coaches and fare rates.
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

	stopTimes, err := h.Queries.ListTripStopTimes(ctx, trip.ID)
	if err != nil {
		log.Printf("GetTrip ListTripStopTimes error: %v (%T)", err, err)
		http.Error(w, "failed to load stop times", http.StatusInternalServerError)
		return
	}
	stopTimeByRouteStationID := make(map[int32]generated.ListTripStopTimesRow, len(stopTimes))
	for _, st := range stopTimes {
		stopTimeByRouteStationID[st.RouteStationID] = st
	}

	tripResp := toTripResponse(trip)
	stationResponses := make([]tripStationResponse, len(stations))
	for i, s := range stations {
		distance, err := numericToFloat64(s.DistanceFromOrigin)
		if err != nil {
			http.Error(w, "invalid distance value", http.StatusInternalServerError)
			return
		}
		resp := tripStationResponse{
			ID:                 s.ID,
			StationID:          s.StationID,
			StationName:        s.StationName,
			StopSequence:       s.StopSequence,
			DistanceFromOrigin: distance,
		}
		switch {
		case i == 0:
			dep := tripResp.DepartureTime
			resp.DepartureTime = &dep
		case i == len(stations)-1:
			arr := tripResp.ArrivalTime
			resp.ArrivalTime = &arr
		default:
			if st, ok := stopTimeByRouteStationID[s.ID]; ok {
				arr := st.ArrivalTime.Time.Format("15:04")
				dep := st.DepartureTime.Time.Format("15:04")
				resp.ArrivalTime = &arr
				resp.DepartureTime = &dep
			}
		}
		stationResponses[i] = resp
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
		"trip":     tripResp,
		"stations": stationResponses,
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

	resp := make([]searchTripResponse, 0, len(trips))
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
		resp = append(resp, searchTripResponse{
			tripResponse:      tr,
			BoardingArrival:   t.BoardingArrival.Time.Format("15:04"),
			BoardingDeparture: t.BoardingDeparture.Time.Format("15:04"),
			AlightingArrival:  t.AlightingArrival.Time.Format("15:04"),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(resp, page, pageSize, total))
}
