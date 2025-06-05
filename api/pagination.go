package api

import (
	"math"
	"strconv"

	"github.com/emprius/emprius-app-backend/db"
)

// GetPaginationParams extracts pagination parameters from the request
func (h *HTTPContext) GetPaginationParams() (page, pageSize int, err error) {
	page, err = h.GetPage()
	if err != nil {
		return 0, 0, err
	}

	pageSize = db.DefaultPageSize
	if pageSizeParam := h.URLParam("pageSize"); pageSizeParam != nil {
		if size, err := strconv.Atoi(pageSizeParam[0]); err == nil && size > 0 {
			pageSize = size
		}
	}

	return page, pageSize, nil
}

// CalculatePagination computes pagination metadata
func CalculatePagination(page, pageSize int, total int64) PaginationInfo {
	totalPages := int(math.Ceil(float64(total) / float64(pageSize)))
	return PaginationInfo{
		Current:  page,
		PageSize: pageSize,
		Total:    total,
		Pages:    totalPages,
	}
}

// GetSearchTerm Get search term if provided
func (h *HTTPContext) GetSearchTerm() *string {
	searchTerm := ""
	if searchParam := h.URLParam("term"); searchParam != nil {
		searchTerm = searchParam[0]
	}
	return &searchTerm
}

// getToolListPaginatedResponse converts DB tools to API tools and creates a paginated response
func (a *API) getToolListPaginatedResponse(tools []*db.Tool, page int, pageSize int, total int64) *PaginatedToolsResponse {
	// Convert DB tools to API tools
	apiTools := make([]*Tool, len(tools))
	for i, t := range tools {
		apiTools[i] = new(Tool).FromDBTool(t)
	}

	// Calculate pagination info
	pagination := CalculatePagination(page, pageSize, total)

	// Create response with pagination info
	return &PaginatedToolsResponse{
		Tools:      apiTools,
		Pagination: pagination,
	}
}

// getBookingListPaginatedResponse converts DB bookings to API bookings and creates a paginated response
func (a *API) getBookingListPaginatedResponse(
	bookings []*db.Booking,
	page int,
	pageSize int,
	total int64,
	authenticatedUserID string,
) *PaginatedBookingsResponse {
	// Convert DB bookings to API bookings
	apiBookings := make([]*BookingResponse, len(bookings))
	for i, b := range bookings {
		apiBookings[i] = a.convertBookingToResponse(b, authenticatedUserID)
	}

	// Calculate pagination info
	pagination := CalculatePagination(page, pageSize, total)

	// Create response with pagination info
	return &PaginatedBookingsResponse{
		Bookings:   apiBookings,
		Pagination: pagination,
	}
}
