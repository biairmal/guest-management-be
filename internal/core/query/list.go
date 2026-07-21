// Package query provides a shared allow-list-based parser for HTTP list-endpoint
// query parameters (pagination, sorting, and simple equality filters), so a
// feature only needs to declare its allow-lists instead of reimplementing parsing.
package query

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	common "github.com/biairmal/go-sdk/lib/common/dto"
)

// Default pagination values applied when a ListParseConfig leaves the
// corresponding field unset (zero value).
const (
	DefaultPage    = 1
	DefaultSize    = 20
	DefaultMaxSize = 100
)

// ListParseConfig configures allow-listed sort/filter fields and pagination
// bounds for ParseListParams. A feature typically only needs to set
// AllowedSortFields and AllowedFilterFields; DefaultPage, DefaultSize, and
// MaxSize fall back to the package-level defaults above when left zero.
type ListParseConfig struct {
	DefaultPage         int
	DefaultSize         int
	MaxSize             int
	AllowedSortFields   []string
	AllowedFilterFields []string
}

// withDefaults returns a copy of c with zero-valued pagination fields filled
// in from the package defaults.
func (c ListParseConfig) withDefaults() ListParseConfig {
	if c.DefaultPage <= 0 {
		c.DefaultPage = DefaultPage
	}
	if c.DefaultSize <= 0 {
		c.DefaultSize = DefaultSize
	}
	if c.MaxSize <= 0 {
		c.MaxSize = DefaultMaxSize
	}
	return c
}

// ListParams is the parsed result of ParseListParams: pagination and sorting
// (via the embedded common.BasePageRequest) plus simple equality filters,
// shared by every list endpoint so no feature needs its own params type.
type ListParams struct {
	common.BasePageRequest
	Filters map[string]string // field -> value (simple equality filters)
}

// ParseListParams parses pagination, sort, and equality-filter query
// parameters per an allow-list config shared by every list endpoint.
//
// Expected query format:
//
//	name=Event1&page=1&size=20&sort=column1,DESC&sort=column2,ASC
//
// - page: 1-based page number (int, defaults to cfg.DefaultPage).
// - size: items per page (int, defaults to cfg.DefaultSize, clamped to cfg.MaxSize).
// - sort: repeatable, format "field,DIRECTION" where DIRECTION is ASC or DESC (case-insensitive).
// - Any key matching cfg.AllowedFilterFields is treated as a simple equality filter.
func ParseListParams(q url.Values, cfg ListParseConfig) (*ListParams, error) {
	cfg = cfg.withDefaults()

	page, err := parsePage(q, cfg)
	if err != nil {
		return nil, err
	}
	size, err := parseSize(q, cfg)
	if err != nil {
		return nil, err
	}
	sorts, err := parseSorts(q, cfg)
	if err != nil {
		return nil, err
	}

	return &ListParams{
		BasePageRequest: *common.NewBasePageRequest(page, size, sorts),
		Filters:         parseFilters(q, cfg),
	}, nil
}

// parsePage parses the "page" query parameter, defaulting to cfg.DefaultPage.
func parsePage(q url.Values, cfg ListParseConfig) (int, error) {
	v := q.Get("page")
	if v == "" {
		return cfg.DefaultPage, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("invalid page value: %s", v)
	}
	return n, nil
}

// parseSize parses the "size" query parameter, defaulting to cfg.DefaultSize
// and clamping to cfg.MaxSize.
func parseSize(q url.Values, cfg ListParseConfig) (int, error) {
	size := cfg.DefaultSize
	if v := q.Get("size"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return 0, fmt.Errorf("invalid size value: %s", v)
		}
		size = n
	}
	if size > cfg.MaxSize {
		size = cfg.MaxSize
	}
	return size, nil
}

// parseSorts parses repeatable "sort=field,DIRECTION" query parameters,
// rejecting fields not in cfg.AllowedSortFields.
func parseSorts(q url.Values, cfg ListParseConfig) ([]common.SortSpec, error) {
	allowedSorts := toSet(cfg.AllowedSortFields)
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
	return sorts, nil
}

// parseFilters extracts simple equality filters for keys in
// cfg.AllowedFilterFields, ignoring pagination/sort keys.
func parseFilters(q url.Values, cfg ListParseConfig) map[string]string {
	allowedFilters := toSet(cfg.AllowedFilterFields)
	filters := make(map[string]string)
	for key := range q {
		if key == "page" || key == "size" || key == "sort" {
			continue
		}
		if allowedFilters[key] {
			filters[key] = q.Get(key)
		}
	}
	return filters
}

// toSet converts a string slice to a set for O(1) lookup.
func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}
