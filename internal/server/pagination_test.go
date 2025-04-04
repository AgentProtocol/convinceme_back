package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestGetPaginationParams(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		queryParams    map[string]string
		expectedParams PaginationParams
	}{
		{
			name:        "Default values",
			queryParams: map[string]string{},
			expectedParams: PaginationParams{
				Page:     1,
				PageSize: DefaultPageSize,
			},
		},
		{
			name: "Custom page and page_size",
			queryParams: map[string]string{
				"page":      "2",
				"page_size": "20",
			},
			expectedParams: PaginationParams{
				Page:     2,
				PageSize: 20,
			},
		},
		{
			name: "Invalid page (negative)",
			queryParams: map[string]string{
				"page": "-1",
			},
			expectedParams: PaginationParams{
				Page:     1, // Should default to 1
				PageSize: DefaultPageSize,
			},
		},
		{
			name: "Invalid page_size (negative)",
			queryParams: map[string]string{
				"page_size": "-10",
			},
			expectedParams: PaginationParams{
				Page:     1,
				PageSize: DefaultPageSize, // Should default to DefaultPageSize
			},
		},
		{
			name: "Page size exceeds maximum",
			queryParams: map[string]string{
				"page_size": "200",
			},
			expectedParams: PaginationParams{
				Page:     1,
				PageSize: MaxPageSize, // Should be capped at MaxPageSize
			},
		},
		{
			name: "Invalid page (non-numeric)",
			queryParams: map[string]string{
				"page": "abc",
			},
			expectedParams: PaginationParams{
				Page:     1, // Should default to 1
				PageSize: DefaultPageSize,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new gin context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create a new request with query parameters
			req := httptest.NewRequest("GET", "/?", nil)
			q := req.URL.Query()
			for key, value := range tc.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			// Set the request to the context
			c.Request = req

			// Call the function
			params := GetPaginationParams(c)

			// Check the results
			assert.Equal(t, tc.expectedParams.Page, params.Page)
			assert.Equal(t, tc.expectedParams.PageSize, params.PageSize)
		})
	}
}

func TestPaginationParamsCalculateOffset(t *testing.T) {
	testCases := []struct {
		name           string
		params         PaginationParams
		expectedOffset int
	}{
		{
			name: "Page 1",
			params: PaginationParams{
				Page:     1,
				PageSize: 10,
			},
			expectedOffset: 0,
		},
		{
			name: "Page 2",
			params: PaginationParams{
				Page:     2,
				PageSize: 10,
			},
			expectedOffset: 10,
		},
		{
			name: "Page 3 with page size 20",
			params: PaginationParams{
				Page:     3,
				PageSize: 20,
			},
			expectedOffset: 40,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			offset := tc.params.CalculateOffset()
			assert.Equal(t, tc.expectedOffset, offset)
		})
	}
}

func TestPaginationParamsCalculateTotalPages(t *testing.T) {
	testCases := []struct {
		name               string
		params             PaginationParams
		expectedTotalPages int
	}{
		{
			name: "No items",
			params: PaginationParams{
				Page:     1,
				PageSize: 10,
				Total:    0,
			},
			expectedTotalPages: 0,
		},
		{
			name: "Exact multiple",
			params: PaginationParams{
				Page:     1,
				PageSize: 10,
				Total:    20,
			},
			expectedTotalPages: 2,
		},
		{
			name: "Partial page",
			params: PaginationParams{
				Page:     1,
				PageSize: 10,
				Total:    25,
			},
			expectedTotalPages: 3,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			totalPages := tc.params.CalculateTotalPages()
			assert.Equal(t, tc.expectedTotalPages, totalPages)
		})
	}
}

func TestBuildPaginationResponse(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	// Create a new gin context
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Test data
	items := []string{"item1", "item2", "item3"}
	params := PaginationParams{
		Page:     2,
		PageSize: 10,
		Total:    25,
	}

	// Call the function
	response := BuildPaginationResponse(c, params, items)

	// Check the results
	assert.Equal(t, items, response["items"])
	pagination, ok := response["pagination"].(gin.H)
	assert.True(t, ok)
	assert.Equal(t, params.Page, pagination["page"])
	assert.Equal(t, params.PageSize, pagination["page_size"])
	assert.Equal(t, params.Total, pagination["total_items"])
	assert.Equal(t, 3, pagination["total_pages"])
	assert.Equal(t, true, pagination["has_prev"])
	assert.Equal(t, true, pagination["has_next"])
}

func TestGetFilterParams(t *testing.T) {
	// Set Gin to test mode
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name           string
		queryParams    map[string]string
		expectedParams FilterParams
	}{
		{
			name:        "Empty parameters",
			queryParams: map[string]string{},
			expectedParams: FilterParams{
				Status:   "",
				Category: "",
				Search:   "",
				SortBy:   "",
				SortDir:  "",
			},
		},
		{
			name: "All parameters",
			queryParams: map[string]string{
				"status":   "active",
				"category": "technology",
				"search":   "test",
				"sort_by":  "created_at",
				"sort_dir": "desc",
			},
			expectedParams: FilterParams{
				Status:   "active",
				Category: "technology",
				Search:   "test",
				SortBy:   "created_at",
				SortDir:  "desc",
			},
		},
		{
			name: "Invalid sort_dir",
			queryParams: map[string]string{
				"sort_dir": "invalid",
			},
			expectedParams: FilterParams{
				Status:   "",
				Category: "",
				Search:   "",
				SortBy:   "",
				SortDir:  "asc", // Should default to "asc"
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new gin context
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Create a new request with query parameters
			req := httptest.NewRequest("GET", "/?", nil)
			q := req.URL.Query()
			for key, value := range tc.queryParams {
				q.Add(key, value)
			}
			req.URL.RawQuery = q.Encode()

			// Set the request to the context
			c.Request = req

			// Call the function
			params := GetFilterParams(c)

			// Check the results
			assert.Equal(t, tc.expectedParams.Status, params.Status)
			assert.Equal(t, tc.expectedParams.Category, params.Category)
			assert.Equal(t, tc.expectedParams.Search, params.Search)
			assert.Equal(t, tc.expectedParams.SortBy, params.SortBy)
			assert.Equal(t, tc.expectedParams.SortDir, params.SortDir)
		})
	}
}

func TestFilterParamsBuildFilterClause(t *testing.T) {
	testCases := []struct {
		name              string
		params            FilterParams
		tableName         string
		expectedClause    string
		expectedArgsCount int
	}{
		{
			name:              "Empty parameters",
			params:            FilterParams{},
			tableName:         "topics",
			expectedClause:    "",
			expectedArgsCount: 0,
		},
		{
			name: "Status only",
			params: FilterParams{
				Status: "active",
			},
			tableName:         "debates",
			expectedClause:    "WHERE debates.status = ?",
			expectedArgsCount: 1,
		},
		{
			name: "Category only",
			params: FilterParams{
				Category: "technology",
			},
			tableName:         "topics",
			expectedClause:    "WHERE topics.category = ?",
			expectedArgsCount: 1,
		},
		{
			name: "Search only",
			params: FilterParams{
				Search: "test",
			},
			tableName:         "topics",
			expectedClause:    "WHERE (topics.title LIKE ? OR topics.description LIKE ?)",
			expectedArgsCount: 2,
		},
		{
			name: "Multiple filters",
			params: FilterParams{
				Status:   "active",
				Category: "technology",
				Search:   "test",
			},
			tableName:         "topics",
			expectedClause:    "WHERE topics.status = ? AND topics.category = ? AND (topics.title LIKE ? OR topics.description LIKE ?)",
			expectedArgsCount: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clause, args := tc.params.BuildFilterClause(tc.tableName)
			assert.Equal(t, tc.expectedClause, clause)
			assert.Equal(t, tc.expectedArgsCount, len(args))
		})
	}
}

func TestFilterParamsBuildOrderClause(t *testing.T) {
	testCases := []struct {
		name           string
		params         FilterParams
		tableName      string
		defaultSort    string
		expectedClause string
	}{
		{
			name:           "Empty parameters",
			params:         FilterParams{},
			tableName:      "topics",
			defaultSort:    "created_at",
			expectedClause: "ORDER BY topics.created_at DESC",
		},
		{
			name: "Sort by with default direction",
			params: FilterParams{
				SortBy: "title",
			},
			tableName:      "topics",
			defaultSort:    "created_at",
			expectedClause: "ORDER BY topics.title ASC",
		},
		{
			name: "Sort by with ascending direction",
			params: FilterParams{
				SortBy:  "title",
				SortDir: "asc",
			},
			tableName:      "topics",
			defaultSort:    "created_at",
			expectedClause: "ORDER BY topics.title ASC",
		},
		{
			name: "Sort by with descending direction",
			params: FilterParams{
				SortBy:  "title",
				SortDir: "desc",
			},
			tableName:      "topics",
			defaultSort:    "created_at",
			expectedClause: "ORDER BY topics.title DESC",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			clause := tc.params.BuildOrderClause(tc.tableName, tc.defaultSort)
			assert.Equal(t, tc.expectedClause, clause)
		})
	}
}
