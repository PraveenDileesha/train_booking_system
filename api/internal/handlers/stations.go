package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"github.com/PraveenDileesha/train_booking_system/internal/apierror"
	"github.com/PraveenDileesha/train_booking_system/internal/generated"
)

// StationHandler serves station creation, listing and deletion endpoints.
type StationHandler struct {
	Queries *generated.Queries
}

type createStationRequest struct {
	Name string `json:"name"`
}

// CreateStation creates a new station.
func (h *StationHandler) CreateStation(w http.ResponseWriter, r *http.Request) {
	var req createStationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	station, err := h.Queries.CreateStation(r.Context(), req.Name)
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("CreateStation unmapped error: %v (%T)", err, err)
		http.Error(w, "failed to create station", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(station)
}

// ListStations defaults to numeric ID order (what the admin stations table wants).
// Pass ?sort=name for alphabetical order, used by every station picker or dropdown outside that table, where a user is scanning for a name rather than tracking IDs.
func (h *StationHandler) ListStations(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)
	limit, offset := int32(pageSize), int32((page-1)*pageSize)

	var stations any
	var err error
	if r.URL.Query().Get("sort") == "name" {
		rows, e := h.Queries.ListStationsByName(r.Context(), generated.ListStationsByNameParams{
			RowLimit:  limit,
			RowOffset: offset,
		})
		if rows == nil {
			rows = []generated.ListStationsByNameRow{}
		}
		stations, err = rows, e
	} else {
		rows, e := h.Queries.ListStations(r.Context(), generated.ListStationsParams{
			RowLimit:  limit,
			RowOffset: offset,
		})
		if rows == nil {
			rows = []generated.ListStationsRow{}
		}
		stations, err = rows, e
	}
	if err != nil {
		log.Printf("ListStations error: %v (%T)", err, err)
		http.Error(w, "failed to list stations", http.StatusInternalServerError)
		return
	}

	total, err := h.Queries.CountStations(r.Context())
	if err != nil {
		log.Printf("CountStations error: %v (%T)", err, err)
		http.Error(w, "failed to list stations", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(stations, page, pageSize, total))
}

// GetStation returns a single station by ID.
func (h *StationHandler) GetStation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid station id", http.StatusBadRequest)
		return
	}

	station, err := h.Queries.GetStation(r.Context(), int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "station not found", http.StatusNotFound)
			return
		}
		log.Printf("GetStation error: %v (%T)", err, err)
		http.Error(w, "failed to get station", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(station)
}

// DeleteStation deletes a station by ID.
func (h *StationHandler) DeleteStation(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid station id", http.StatusBadRequest)
		return
	}

	rowsAffected, err := h.Queries.DeleteStation(r.Context(), int32(id))
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("DeleteStation unmapped error: %v (%T)", err, err)
		http.Error(w, "failed to delete station", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "station not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
