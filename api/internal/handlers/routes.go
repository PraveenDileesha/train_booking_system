package handlers

import (
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

type RouteHandler struct {
	Pool    *pgxpool.Pool
	Queries *generated.Queries
}

type createRouteStationInput struct {
	StationID          int32   `json:"station_id"`
	DistanceFromOrigin float64 `json:"distance_from_origin"`
}

type createRouteRequest struct {
	Name     string                    `json:"name"`
	Stations []createRouteStationInput `json:"stations"`
}

func numericFromFloat64(f float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(f, 'f', 2, 64))
	return n, err
}

func numericToFloat64(n pgtype.Numeric) (float64, error) {
	f8, err := n.Float64Value()
	if err != nil {
		return 0, err
	}
	if !f8.Valid {
		return 0, fmt.Errorf("numeric value is not valid")
	}
	return f8.Float64, nil
}

func nextVersionNo(current pgtype.Numeric) (pgtype.Numeric, error) {
	f, err := numericToFloat64(current)
	if err != nil {
		return pgtype.Numeric{}, err
	}

	major := math.Floor(f + 1e-9)
	minor := math.Round((f-major)*10 + 1e-9)

	if minor >= 9 {
		major++
		minor = 0
	} else {
		minor++
	}

	return numericFromFloat64(major + minor/10)
}

type routeVersionResponse struct {
	ID        int32     `json:"id"`
	RouteID   int32     `json:"route_id"`
	VersionNo string    `json:"version_no"`
	CreatedAt time.Time `json:"created_at"`
	IsActive  bool      `json:"is_active"`
}

func toRouteVersionResponse(v generated.RouteVersion) (routeVersionResponse, error) {
	f, err := numericToFloat64(v.VersionNo)
	if err != nil {
		return routeVersionResponse{}, err
	}
	return routeVersionResponse{
		ID:        v.ID,
		RouteID:   v.RouteID,
		VersionNo: fmt.Sprintf("%.1f", f),
		CreatedAt: v.CreatedAt.Time,
		IsActive:  v.IsActive,
	}, nil
}

// CreateRoute creates a route, its first version, and that version's stations.
func (h *RouteHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	var req createRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(req.Stations) < 2 {
		http.Error(w, "a route must have at least 2 stations (start and end)", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)

	route, err := qtx.CreateRoute(ctx, req.Name)
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		http.Error(w, "failed to create route", http.StatusInternalServerError)
		return
	}

	versionNo, err := numericFromFloat64(1.0)
	if err != nil {
		http.Error(w, "failed to construct version number", http.StatusInternalServerError)
		return
	}

	version, err := qtx.CreateRouteVersion(ctx, generated.CreateRouteVersionParams{
		RouteID:   route.ID,
		VersionNo: versionNo,
		IsActive:  true,
	})
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		http.Error(w, "failed to create route version", http.StatusInternalServerError)
		return
	}

	for i, station := range req.Stations {
		distance, err := numericFromFloat64(station.DistanceFromOrigin)
		if err != nil {
			http.Error(w, "invalid distance_from_origin value", http.StatusBadRequest)
			return
		}

		_, err = qtx.CreateRouteStation(ctx, generated.CreateRouteStationParams{
			RouteVersionID:     version.ID,
			StationID:          station.StationID,
			StopSequence:       int32(i),
			DistanceFromOrigin: distance,
		})
		if err != nil {
			if apierror.WritePostgresError(w, err) {
				return
			}
			http.Error(w, "failed to add station to route", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to save route", http.StatusInternalServerError)
		return
	}

	stations, err := h.Queries.GetRouteVersionStations(ctx, version.ID)
	if err != nil {
		http.Error(w, "route created but failed to load stations", http.StatusInternalServerError)
		return
	}

	versionResp, err := toRouteVersionResponse(version)
	if err != nil {
		http.Error(w, "route created but failed to format version", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"route":    route,
		"version":  versionResp,
		"stations": stations,
	})
}

// ListRoutes returns a paginated list of routes.
func (h *RouteHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)

	routes, err := h.Queries.ListRoutes(r.Context(), generated.ListRoutesParams{
		RowLimit:  int32(pageSize),
		RowOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		http.Error(w, "failed to list routes", http.StatusInternalServerError)
		return
	}

	total, err := h.Queries.CountRoutes(r.Context())
	if err != nil {
		http.Error(w, "failed to list routes", http.StatusInternalServerError)
		return
	}

	if routes == nil {
		routes = []generated.Route{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(routes, page, pageSize, total))
}

// GetRoute returns a route and its active version's stations by ID.
func (h *RouteHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid route id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	route, err := h.Queries.GetRoute(ctx, int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "route not found", http.StatusNotFound)
			return
		}
		http.Error(w, "failed to get route", http.StatusInternalServerError)
		return
	}

	version, err := h.Queries.GetActiveRouteVersion(ctx, int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// route exists but has no active version (shouldn't normally happen, but don't fail the whole request over it)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"route":    route,
				"version":  nil,
				"stations": []any{},
			})
			return
		}
		log.Printf("GetRoute GetActiveRouteVersion error: %v (%T)", err, err)
		http.Error(w, "failed to load route version", http.StatusInternalServerError)
		return
	}

	stations, err := h.Queries.GetRouteVersionStations(ctx, version.ID)
	if err != nil {
		log.Printf("GetRoute GetRouteVersionStations error: %v (%T)", err, err)
		http.Error(w, "failed to load stations", http.StatusInternalServerError)
		return
	}

	versionResp, err := toRouteVersionResponse(version)
	if err != nil {
		http.Error(w, "failed to format version", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"route":    route,
		"version":  versionResp,
		"stations": stations,
	})
}

// DeleteRoute deletes a route by ID.
func (h *RouteHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid route id", http.StatusBadRequest)
		return
	}

	rowsAffected, err := h.Queries.DeleteRoute(r.Context(), int32(id))
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		http.Error(w, "failed to delete route", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "route not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateRouteStations creates a new immutable route_version with the given station list and makes it the active version, deactivating whatever version was active before.
// Nothing is ever mutated in place.
func (h *RouteHandler) UpdateRouteStations(w http.ResponseWriter, r *http.Request) {
	routeID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid route id", http.StatusBadRequest)
		return
	}

	var req createRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if len(req.Stations) < 2 {
		http.Error(w, "a route must have at least 2 stations (start and end)", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)

	currentVersion, err := qtx.GetActiveRouteVersion(ctx, int32(routeID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "route not found or has no active version", http.StatusNotFound)
			return
		}
		log.Printf("UpdateRouteStations GetActiveRouteVersion error: %v (%T)", err, err)
		http.Error(w, "failed to load current route version", http.StatusInternalServerError)
		return
	}

	nextNo, err := nextVersionNo(currentVersion.VersionNo)
	if err != nil {
		log.Printf("UpdateRouteStations nextVersionNo error: %v (%T)", err, err)
		http.Error(w, "failed to compute next version number", http.StatusInternalServerError)
		return
	}

	// Deactivate the old version BEFORE inserting the new active one, since the DB has a partial unique index allowing only one is_active=true row per route_id.
	if err := qtx.DeactivateRouteVersion(ctx, currentVersion.ID); err != nil {
		http.Error(w, "failed to deactivate current version", http.StatusInternalServerError)
		return
	}

	newVersion, err := qtx.CreateRouteVersion(ctx, generated.CreateRouteVersionParams{
		RouteID:   int32(routeID),
		VersionNo: nextNo,
		IsActive:  true,
	})
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("UpdateRouteStations CreateRouteVersion error: %v (%T)", err, err)
		http.Error(w, "failed to create new route version", http.StatusInternalServerError)
		return
	}

	for i, station := range req.Stations {
		distance, err := numericFromFloat64(station.DistanceFromOrigin)
		if err != nil {
			http.Error(w, "invalid distance_from_origin value", http.StatusBadRequest)
			return
		}

		_, err = qtx.CreateRouteStation(ctx, generated.CreateRouteStationParams{
			RouteVersionID:     newVersion.ID,
			StationID:          station.StationID,
			StopSequence:       int32(i),
			DistanceFromOrigin: distance,
		})
		if err != nil {
			if apierror.WritePostgresError(w, err) {
				return
			}
			http.Error(w, "failed to add station to new route version", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to save new route version", http.StatusInternalServerError)
		return
	}

	stations, err := h.Queries.GetRouteVersionStations(ctx, newVersion.ID)
	if err != nil {
		http.Error(w, "route version created but failed to load stations", http.StatusInternalServerError)
		return
	}

	versionResp, err := toRouteVersionResponse(newVersion)
	if err != nil {
		http.Error(w, "route version created but failed to format version", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"version":  versionResp,
		"stations": stations,
	})
}
