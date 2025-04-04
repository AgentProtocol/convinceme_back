package server

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// PaginationParams contains pagination parameters
type PaginationParams struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total,omitempty"`
}

// DefaultPageSize is the default number of items per page
const DefaultPageSize = 10

// MaxPageSize is the maximum allowed page size
const MaxPageSize = 100

// GetPaginationParams extracts pagination parameters from the request
func GetPaginationParams(c *gin.Context) PaginationParams {
	// Default values
	params := PaginationParams{
		Page:     1,
		PageSize: DefaultPageSize,
	}

	// Parse page parameter
	if pageStr := c.Query("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			params.Page = page
		}
	}

	// Parse page_size parameter
	if pageSizeStr := c.Query("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 {
			// Limit page size to prevent excessive queries
			if pageSize > MaxPageSize {
				pageSize = MaxPageSize
			}
			params.PageSize = pageSize
		}
	}

	return params
}

// CalculateOffset calculates the offset for SQL queries based on pagination parameters
func (p PaginationParams) CalculateOffset() int {
	return (p.Page - 1) * p.PageSize
}

// CalculateTotalPages calculates the total number of pages based on total items
func (p PaginationParams) CalculateTotalPages() int {
	if p.Total == 0 {
		return 0
	}
	totalPages := p.Total / p.PageSize
	if p.Total%p.PageSize > 0 {
		totalPages++
	}
	return totalPages
}

// BuildPaginationResponse builds a standardized pagination response
func BuildPaginationResponse(c *gin.Context, params PaginationParams, items any) gin.H {
	totalPages := params.CalculateTotalPages()

	return gin.H{
		"items": items,
		"pagination": gin.H{
			"page":        params.Page,
			"page_size":   params.PageSize,
			"total_items": params.Total,
			"total_pages": totalPages,
			"has_next":    params.Page < totalPages,
			"has_prev":    params.Page > 1,
		},
	}
}

// SendPaginatedResponse sends a paginated response
func SendPaginatedResponse(c *gin.Context, params PaginationParams, items any) {
	c.JSON(http.StatusOK, BuildPaginationResponse(c, params, items))
}

// FilterParams contains common filter parameters
type FilterParams struct {
	Status   string `form:"status"`
	Category string `form:"category"`
	Search   string `form:"search"`
	SortBy   string `form:"sort_by"`
	SortDir  string `form:"sort_dir"` // asc or desc
}

// GetFilterParams extracts filter parameters from the request
func GetFilterParams(c *gin.Context) FilterParams {
	var params FilterParams

	// Bind query parameters to struct
	if err := c.ShouldBindQuery(&params); err != nil {
		// Just use default values if binding fails
		return FilterParams{}
	}

	// Validate sort direction
	if params.SortDir != "" && params.SortDir != "asc" && params.SortDir != "desc" {
		params.SortDir = "asc" // Default to ascending if invalid
	}

	return params
}

// BuildFilterClause builds a SQL WHERE clause based on filter parameters
func (f FilterParams) BuildFilterClause(tableName string) (string, []any) {
	whereClause := ""
	args := []any{}

	// Add status filter if provided
	if f.Status != "" {
		if whereClause != "" {
			whereClause += " AND "
		}
		whereClause += fmt.Sprintf("%s.status = ?", tableName)
		args = append(args, f.Status)
	}

	// Add category filter if provided
	if f.Category != "" {
		if whereClause != "" {
			whereClause += " AND "
		}
		whereClause += fmt.Sprintf("%s.category = ?", tableName)
		args = append(args, f.Category)
	}

	// Add search filter if provided
	if f.Search != "" {
		if whereClause != "" {
			whereClause += " AND "
		}
		// Search in title or description
		whereClause += fmt.Sprintf("(%s.title LIKE ? OR %s.description LIKE ?)", tableName, tableName)
		searchTerm := "%" + f.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Prepend WHERE if we have conditions
	if whereClause != "" {
		whereClause = "WHERE " + whereClause
	}

	return whereClause, args
}

// BuildOrderClause builds a SQL ORDER BY clause based on filter parameters
func (f FilterParams) BuildOrderClause(tableName string, defaultSort string) string {
	if f.SortBy == "" {
		return fmt.Sprintf("ORDER BY %s.%s %s", tableName, defaultSort, "DESC")
	}

	direction := "ASC"
	if f.SortDir == "desc" {
		direction = "DESC"
	}

	return fmt.Sprintf("ORDER BY %s.%s %s", tableName, f.SortBy, direction)
}
