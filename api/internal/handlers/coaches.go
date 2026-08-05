package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PraveenDileesha/train_booking_system/internal/apierror"
	"github.com/PraveenDileesha/train_booking_system/internal/generated"
)

type CoachHandler struct {
	Pool    *pgxpool.Pool
	Queries *generated.Queries
}

// seatLetters returns the seat letters for one row of the given class, following real Sri Lanka Railways carriage layouts. First and Second class run 2+2 (four seats per row) and Third class runs 3+2 (five seats per row).
func seatLetters(class generated.CoachClass) []string {
	switch class {
	case generated.CoachClassTHIRD:
		return []string{"A", "B", "C", "D", "E"}
	default: // FIRST_AC, SECOND
		return []string{"A", "B", "C", "D"}
	}
}

type createCoachRequest struct {
	CoachName    string               `json:"coach_name"`
	Class        generated.CoachClass `json:"class"`
	IsReservable bool                 `json:"is_reservable"`
	RowCount     int32                `json:"row_count"`
}

func validCoachClass(c generated.CoachClass) bool {
	switch c {
	case generated.CoachClassFIRSTAC, generated.CoachClassSECOND, generated.CoachClassTHIRD:
		return true
	}
	return false
}

// CreateCoach creates a coach and auto-generates its seats from the class's row layout, so an admin only ever enters a row count, never individual seat numbers.
func (h *CoachHandler) CreateCoach(w http.ResponseWriter, r *http.Request) {
	var req createCoachRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.CoachName == "" {
		http.Error(w, "coach_name is required", http.StatusBadRequest)
		return
	}
	if !validCoachClass(req.Class) {
		http.Error(w, "class must be one of FIRST_AC, SECOND, THIRD", http.StatusBadRequest)
		return
	}
	if req.RowCount < 1 {
		http.Error(w, "row_count must be at least 1", http.StatusBadRequest)
		return
	}

	letters := seatLetters(req.Class)
	capacity := req.RowCount * int32(len(letters))

	ctx := r.Context()
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		http.Error(w, "failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback(ctx)

	qtx := h.Queries.WithTx(tx)

	coach, err := qtx.CreateCoach(ctx, generated.CreateCoachParams{
		CoachName:    req.CoachName,
		Class:        req.Class,
		IsReservable: req.IsReservable,
		RowCount:     req.RowCount,
		Capacity:     capacity,
	})
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("CreateCoach CreateCoach error: %v (%T)", err, err)
		http.Error(w, "failed to create coach", http.StatusInternalServerError)
		return
	}

	seatParams := make([]generated.CreateSeatsParams, 0, capacity)
	for row := int32(1); row <= req.RowCount; row++ {
		for _, letter := range letters {
			seatParams = append(seatParams, generated.CreateSeatsParams{
				CoachID:    coach.ID,
				SeatNumber: strconv.Itoa(int(row)) + letter,
			})
		}
	}
	if _, err := qtx.CreateSeats(ctx, seatParams); err != nil {
		log.Printf("CreateCoach CreateSeats error: %v (%T)", err, err)
		http.Error(w, "failed to generate seats", http.StatusInternalServerError)
		return
	}

	seats, err := qtx.ListSeatsByCoach(ctx, coach.ID)
	if err != nil {
		log.Printf("CreateCoach ListSeatsByCoach error: %v (%T)", err, err)
		http.Error(w, "failed to load generated seats", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(ctx); err != nil {
		http.Error(w, "failed to save coach", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"coach": coach,
		"seats": seats,
	})
}

// ListCoaches returns a paginated list of coaches.
func (h *CoachHandler) ListCoaches(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePagination(r)

	coaches, err := h.Queries.ListCoaches(r.Context(), generated.ListCoachesParams{
		RowLimit:  int32(pageSize),
		RowOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		log.Printf("ListCoaches error: %v (%T)", err, err)
		http.Error(w, "failed to list coaches", http.StatusInternalServerError)
		return
	}

	total, err := h.Queries.CountCoaches(r.Context())
	if err != nil {
		log.Printf("CountCoaches error: %v (%T)", err, err)
		http.Error(w, "failed to list coaches", http.StatusInternalServerError)
		return
	}

	if coaches == nil {
		coaches = []generated.ListCoachesRow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(newPaginatedResponse(coaches, page, pageSize, total))
}

// GetCoach returns a single coach and its seats by ID.
func (h *CoachHandler) GetCoach(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid coach id", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	coach, err := h.Queries.GetCoach(ctx, int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "coach not found", http.StatusNotFound)
			return
		}
		log.Printf("GetCoach error: %v (%T)", err, err)
		http.Error(w, "failed to get coach", http.StatusInternalServerError)
		return
	}

	seats, err := h.Queries.ListSeatsByCoach(ctx, coach.ID)
	if err != nil {
		log.Printf("GetCoach ListSeatsByCoach error: %v (%T)", err, err)
		http.Error(w, "failed to load seats", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"coach": coach,
		"seats": seats,
	})
}

// DeleteCoach deletes a coach and its seats by ID.
func (h *CoachHandler) DeleteCoach(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid coach id", http.StatusBadRequest)
		return
	}

	rowsAffected, err := h.Queries.DeleteCoach(r.Context(), int32(id))
	if err != nil {
		if apierror.WritePostgresError(w, err) {
			return
		}
		log.Printf("DeleteCoach unmapped error: %v (%T)", err, err)
		http.Error(w, "failed to delete coach", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		http.Error(w, "coach not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
