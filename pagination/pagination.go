package pagination

import "math"

// Query defines the query parameter inputs for pagination.
// Struct tags use `form` to map directly to URL query strings (e.g., ?page=1&limit=20).
type Query struct {
	Page  int `form:"page,default=1" binding:"omitempty,min=1"`
	Limit int `form:"limit,default=10" binding:"omitempty,min=1,max=100"`
}

// EnsureDefaults sets fallback values if page or limit are zero/unspecified.
func (q *Query) EnsureDefaults() {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.Limit > 100 {
		q.Limit = 100
	}
	if q.Limit <= 0 {
		q.Limit = 10
	}
}

// Offset calculates the SQL/Database offset.
func (q Query) Offset() int {
	q.EnsureDefaults()
	if q.Page-1 > math.MaxInt/q.Limit {
		return math.MaxInt
	}
	return (q.Page - 1) * q.Limit
}

// Meta holds pagination response metadata.
type Meta struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalItems int `json:"total_items"`
	TotalPages int `json:"total_pages"`
}

// Result wraps paginated data items along with metadata.
type Result[T any] struct {
	Items []T  `json:"items"`
	Meta  Meta `json:"meta"`
}

// NewResult constructs a Result[T] with calculated total pages.
func NewResult[T any](items []T, totalItems int, q Query) Result[T] {
	q.EnsureDefaults()
	if items == nil {
		items = []T{}
	}
	totalPages := 0
	if q.Limit > 0 {
		if totalItems < 0 {
			totalItems = 0
		}
		totalPages = totalItems / q.Limit
		if totalItems%q.Limit != 0 {
			totalPages++
		}
	}

	if items == nil {
		items = []T{} // Ensure empty slice rather than null in JSON
	}

	return Result[T]{
		Items: items,
		Meta: Meta{
			Page:       q.Page,
			Limit:      q.Limit,
			TotalItems: totalItems,
			TotalPages: totalPages,
		},
	}
}
