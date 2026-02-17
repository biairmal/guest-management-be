package events

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	common "github.com/biairmal/go-sdk/common/dto"
)

// EventCategoryListParams holds list parameters for event categories.
// Embeds common.BasePageRequest for pagination and sorting.
type EventCategoryListParams struct {
	common.BasePageRequest
	Filters map[string]string // field -> value (simple equality filters)
}

// listParseConfig configures allowed sort and filter fields for list query params.
type listParseConfig struct {
	DefaultPage         int
	DefaultSize         int
	MaxSize             int
	AllowedSortFields   []string
	AllowedFilterFields []string
}

// eventCategoryListConfig is the parse configuration for event category list.
var eventCategoryListConfig = listParseConfig{
	DefaultPage:         1,
	DefaultSize:         20,
	MaxSize:             100,
	AllowedSortFields:   []string{"id", "source", "tenant_id", "name", "created_at", "updated_at"},
	AllowedFilterFields: []string{"name", "source", "tenant_id"},
}

// ParseEventCategoryListParams parses URL query parameters into EventCategoryListParams.
//
// Expected query format:
//
//	name=Event1&page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// - page: 1-based page number (int).
// - size: items per page (int, clamped to MaxSize).
// - sort: repeatable, format "field,DIRECTION" where DIRECTION is ASC or DESC (case-insensitive).
// - Any key matching AllowedFilterFields is treated as a simple equality filter.
func ParseEventCategoryListParams(q url.Values, config listParseConfig) (*EventCategoryListParams, error) {
	// Parse page.
	page := config.DefaultPage
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid page value: %s", v)
		}
		page = n
	}

	// Parse size.
	size := config.DefaultSize
	if v := q.Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("invalid size value: %s", v)
		}
		size = n
	}
	if size > config.MaxSize {
		size = config.MaxSize
	}

	// Parse sorts: sort=field,DIRECTION (repeatable).
	allowedSorts := toSet(config.AllowedSortFields)
	var sorts []common.SortSpec
	for _, sv := range q["sort"] {
		parts := strings.SplitN(sv, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid sort format: %s (expected field,DIRECTION)", sv)
		}
		field := strings.TrimSpace(parts[0])
		dirStr := strings.ToUpper(strings.TrimSpace(parts[1]))

		if !allowedSorts[field] {
			return nil, fmt.Errorf("sort field not allowed: %s", field)
		}
		var dir common.SortDirection
		switch dirStr {
		case string(common.SortAsc):
			dir = common.SortAsc
		case string(common.SortDesc):
			dir = common.SortDesc
		default:
			return nil, fmt.Errorf("invalid sort direction: %s (expected ASC or DESC)", parts[1])
		}
		sorts = append(sorts, common.SortSpec{Field: field, Direction: dir})
	}

	// Parse filters: keys that match AllowedFilterFields (simple equality, first value).
	allowedFilters := toSet(config.AllowedFilterFields)
	filters := make(map[string]string)
	for key := range q {
		if key == "page" || key == "size" || key == "sort" {
			continue
		}
		if allowedFilters[key] {
			filters[key] = q.Get(key)
		}
	}

	return &EventCategoryListParams{
		BasePageRequest: *common.NewBasePageRequest(page, size, sorts),
		Filters:         filters,
	}, nil
}

// toSet converts a string slice to a set for O(1) lookup.
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
