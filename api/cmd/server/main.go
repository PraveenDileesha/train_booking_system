package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"golang.org/x/time/rate"

	"github.com/PraveenDileesha/train_booking_system/internal/generated"
	"github.com/PraveenDileesha/train_booking_system/internal/handlers"
	"github.com/PraveenDileesha/train_booking_system/internal/middleware"
	"github.com/PraveenDileesha/train_booking_system/internal/postgres"
)

func main() {
	ctx := context.Background()

	pool, err := postgres.New(ctx)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("connected to database")

	queries := generated.New(pool)
	stationHandler := &handlers.StationHandler{Queries: queries}
	routeHandler := &handlers.RouteHandler{Pool: pool, Queries: queries}
	coachHandler := &handlers.CoachHandler{Pool: pool, Queries: queries}
	tripHandler := &handlers.TripHandler{Pool: pool, Queries: queries}
	bookingHandler := &handlers.BookingHandler{Pool: pool, Queries: queries}
	counterHandler := &handlers.CounterHandler{Pool: pool, Queries: queries}
	revenueHandler := &handlers.RevenueHandler{Queries: queries}

	mux := http.NewServeMux()

	// Public (non-admin) endpoints are the ones exposed to anonymous customers and the ticket counter. 5 requests per second sustained per IP, bursts up to 20.
	// Admin endpoints are trusted internal tooling and aren't rate limited.
	publicLimiter := middleware.NewIPRateLimiter(rate.Limit(5), 20)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unreachable", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})

	mux.HandleFunc("POST /api/v1/admin/stations", stationHandler.CreateStation)
	mux.HandleFunc("GET /api/v1/admin/stations", stationHandler.ListStations)
	mux.HandleFunc("GET /api/v1/admin/stations/{id}", stationHandler.GetStation)
	mux.HandleFunc("DELETE /api/v1/admin/stations/{id}", stationHandler.DeleteStation)

	mux.HandleFunc("POST /api/v1/admin/routes", routeHandler.CreateRoute)
	mux.HandleFunc("GET /api/v1/admin/routes", routeHandler.ListRoutes)
	mux.HandleFunc("GET /api/v1/admin/routes/{id}", routeHandler.GetRoute)
	mux.HandleFunc("DELETE /api/v1/admin/routes/{id}", routeHandler.DeleteRoute)
	mux.HandleFunc("PUT /api/v1/admin/routes/{id}/stations", routeHandler.UpdateRouteStations)

	mux.HandleFunc("POST /api/v1/admin/coaches", coachHandler.CreateCoach)
	mux.HandleFunc("GET /api/v1/admin/coaches", coachHandler.ListCoaches)
	mux.HandleFunc("GET /api/v1/admin/coaches/{id}", coachHandler.GetCoach)
	mux.HandleFunc("DELETE /api/v1/admin/coaches/{id}", coachHandler.DeleteCoach)

	mux.HandleFunc("POST /api/v1/admin/trips", tripHandler.CreateTrip)
	mux.HandleFunc("GET /api/v1/admin/trips", tripHandler.ListTrips)
	mux.HandleFunc("DELETE /api/v1/admin/trips/{id}", tripHandler.DeleteTrip)

	mux.HandleFunc("GET /api/v1/admin/revenue/today", revenueHandler.GetTodayRevenue)
	mux.HandleFunc("GET /api/v1/admin/revenue/daily", revenueHandler.ListDailyRevenue)
	mux.HandleFunc("GET /api/v1/admin/revenue/bookings", revenueHandler.ListRevenueBookingsByDate)

	// Trip detail, search and booking are shared between the admin and customer-facing UIs, so they live outside /admin.
	// The trip, booking and counter endpoints below are the public surface and get rate limited.
	mux.Handle("GET /api/v1/trips", publicLimiter.Limit(tripHandler.SearchTrips))
	mux.Handle("GET /api/v1/trips/{id}", publicLimiter.Limit(tripHandler.GetTrip))
	mux.Handle("GET /api/v1/trips/{id}/seats", publicLimiter.Limit(bookingHandler.ListAvailableSeats))

	mux.Handle("POST /api/v1/bookings", publicLimiter.Limit(bookingHandler.CreateBooking))
	mux.Handle("GET /api/v1/bookings/{id}", publicLimiter.Limit(bookingHandler.GetBooking))
	mux.Handle("POST /api/v1/bookings/{id}/confirm", publicLimiter.Limit(bookingHandler.ConfirmBooking))

	mux.Handle("POST /api/v1/counter/tickets", publicLimiter.Limit(counterHandler.IssueUnreservedTicket))

	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
