package api

import (
	"github.com/emprius/emprius-app-backend/db"
	"math"
	"strconv"
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
