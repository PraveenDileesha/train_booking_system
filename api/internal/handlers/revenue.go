package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PraveenDileesha/train_booking_system/internal/generated"
)

const (
	defaultRevenueDays = 30
	maxRevenueDays     = 365
)

type RevenueHandler struct {
	Queries *generated.Queries
}

type todayRevenueResponse struct {
	Revenue      float64 `json:"revenue"`
	BookingCount int64   `json:"booking_count"`
}

// GetTodayRevenue returns total confirmed-booking revenue and booking count for the current calendar day.
func (h *RevenueHandler) GetTodayRevenue(w http.ResponseWriter, r *http.Request) {
	row, err := h.Queries.GetTodayRevenue(r.Context())
	if err != nil {
		log.Printf("GetTodayRevenue error: %v (%T)", err, err)
		http.Error(w, "failed to load today's revenue", http.StatusInternalServerError)
		return
	}

	revenue, err := numericToFloat64(row.Revenue)
	if err != nil {
		http.Error(w, "invalid revenue value", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(todayRevenueResponse{
		Revenue:      revenue,
		BookingCount: row.BookingCount,
	})
}

type dailyRevenueEntry struct {
	Date         string  `json:"date"`
	Revenue      float64 `json:"revenue"`
	BookingCount int64   `json:"booking_count"`
}

// ListDailyRevenue returns confirmed-booking revenue grouped by the day it was confirmed, most recent day first.
// Accepts ?days= to control how many days back to include, defaulting to defaultRevenueDays and capped at maxRevenueDays.
func (h *RevenueHandler) ListDailyRevenue(w http.ResponseWriter, r *http.Request) {
	days := defaultRevenueDays
	if v, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && v > 0 {
		days = v
	}
	if days > maxRevenueDays {
		days = maxRevenueDays
	}

	rows, err := h.Queries.ListDailyRevenue(r.Context(), int32(days))
	if err != nil {
		log.Printf("ListDailyRevenue error: %v (%T)", err, err)
		http.Error(w, "failed to load daily revenue", http.StatusInternalServerError)
		return
	}

	entries := make([]dailyRevenueEntry, 0, len(rows))
	for _, row := range rows {
		revenue, err := numericToFloat64(row.Revenue)
		if err != nil {
			http.Error(w, "invalid revenue value", http.StatusInternalServerError)
			return
		}
		entries = append(entries, dailyRevenueEntry{
			Date:         dateToString(row.Day),
			Revenue:      revenue,
			BookingCount: row.BookingCount,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"days": entries})
}

type revenueBookingResponse struct {
	ID               int32   `json:"id"`
	BookingReference string  `json:"booking_reference"`
	Fare             float64 `json:"fare"`
	ConfirmedAt      string  `json:"confirmed_at"`
	PassengerName    string  `json:"passenger_name"`
	SeatNumber       string  `json:"seat_number"`
	CoachName        string  `json:"coach_name"`
	Class            string  `json:"class"`
	TripID           int32   `json:"trip_id"`
	DepartureDate    string  `json:"departure_date"`
	DepartureTime    string  `json:"departure_time"`
	RouteName        string  `json:"route_name"`
	StartStationName string  `json:"start_station_name"`
	EndStationName   string  `json:"end_station_name"`
}

// ListRevenueBookingsByDate returns every confirmed booking for one calendar day, each annotated with the seat, coach, route and trip it was sold against, so the revenue log reads as a real sales record rather than a bare list of booking IDs.
// Requires ?date= (YYYY-MM-DD) and accepts page and page_size.
func (h *RevenueHandler) ListRevenueBookingsByDate(w http.ResponseWriter, r *http.Request) {
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		http.Error(w, "date query param is required (YYYY-MM-DD)", http.StatusBadRequest)
		return
	}
	parsedDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		http.Error(w, "invalid date, expected YYYY-MM-DD", http.StatusBadRequest)
		return
	}
	day := pgtype.Timestamp{Time: parsedDate, Valid: true}

	page, pageSize := parsePagination(r)
	ctx := r.Context()

	rows, err := h.Queries.ListRevenueBookingsByDate(ctx, generated.ListRevenueBookingsByDateParams{
		Day:       day,
		RowLimit:  int32(pageSize),
		RowOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		log.Printf("ListRevenueBookingsByDate error: %v (%T)", err, err)
		http.Error(w, "failed to list bookings", http.StatusInternalServerError)
		return
	}

	total, err := h.Queries.CountRevenueBookingsByDate(ctx, day)
	if err != nil {
		log.Printf("CountRevenueBookingsByDate error: %v (%T)", err, err)
		http.Error(w, "failed to list bookings", http.StatusInternalServerError)
		return
	}

	bookings := make([]revenueBookingResponse, 0, len(rows))
	for _, row := range rows {
		fare, err := numericToFloat64(row.Fare)
		if err != nil {
			http.Error(w, "invalid fare value", http.StatusInternalServerError)
			return
		}
		bookings = append(bookings, revenueBookingResponse{
			ID:               row.ID,
			BookingReference: row.BookingReference,
			Fare:             fare,
			ConfirmedAt:      row.ConfirmedAt.Time.Format(time.RFC3339),
			PassengerName:    row.PassengerName,
			SeatNumber:       row.SeatNumber,
			CoachName:        row.CoachName,
			Class:            string(row.Class),
			TripID:           row.TripID,
			DepartureDate:    dateToString(row.DepartureDate),
			DepartureTime:    timeToString(row.DepartureTime),
			RouteName:        row.RouteName,
			StartStationName: row.StartStationName,
			EndStationName:   row.EndStationName,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(bookings, page, pageSize, total))
}
