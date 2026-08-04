package handlers

import (
	"net/http"
	"strconv"
)

const (
	defaultPageSize = 20
	maxPageSize     = 200
)

// parsePagination reads the "page" and "page_size" query params. page defaults to 1; page_size defaults to defaultPageSize and is clamped to [1, maxPageSize].
func parsePagination(r *http.Request) (page, pageSize int) {
	page = 1
	if v, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && v > 0 {
		page = v
	}

	pageSize = defaultPageSize
	if v, err := strconv.Atoi(r.URL.Query().Get("page_size")); err == nil && v > 0 {
		pageSize = v
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return page, pageSize
}

type paginatedResponse struct {
	Items      any   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}

func newPaginatedResponse(items any, page, pageSize int, total int64) paginatedResponse {
	totalPages := int((total + int64(pageSize) - 1) / int64(pageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	return paginatedResponse{
		Items:      items,
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: totalPages,
	}
}
