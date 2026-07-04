package query

import (
	"net/url"
	"testing"

	common "github.com/biairmal/go-sdk/common/dto"
)

func TestParseListParamsDefaults(t *testing.T) {
	cfg := ListParseConfig{
		AllowedSortFields:   []string{"name"},
		AllowedFilterFields: []string{"name"},
	}

	params, err := ParseListParams(url.Values{}, cfg)
	if err != nil {
		t.Fatalf("ParseListParams() error = %v, want nil", err)
	}
	if params.Page != DefaultPage {
		t.Errorf("Page = %d, want package default %d", params.Page, DefaultPage)
	}
	if params.Size != DefaultSize {
		t.Errorf("Size = %d, want package default %d", params.Size, DefaultSize)
	}
	if len(params.Sorts) != 0 {
		t.Errorf("Sorts = %v, want empty", params.Sorts)
	}
	if len(params.Filters) != 0 {
		t.Errorf("Filters = %v, want empty", params.Filters)
	}
}

func TestParseListParamsOverridesConfigDefaults(t *testing.T) {
	cfg := ListParseConfig{
		DefaultPage: 2,
		DefaultSize: 5,
		MaxSize:     10,
	}

	params, err := ParseListParams(url.Values{}, cfg)
	if err != nil {
		t.Fatalf("ParseListParams() error = %v, want nil", err)
	}
	if params.Page != 2 {
		t.Errorf("Page = %d, want 2 (config default)", params.Page)
	}
	if params.Size != 5 {
		t.Errorf("Size = %d, want 5 (config default)", params.Size)
	}
}

func TestParseListParamsSizeClampedToMax(t *testing.T) {
	cfg := ListParseConfig{MaxSize: 10}
	q := url.Values{"size": {"999"}}

	params, err := ParseListParams(q, cfg)
	if err != nil {
		t.Fatalf("ParseListParams() error = %v, want nil", err)
	}
	if params.Size != 10 {
		t.Errorf("Size = %d, want clamped to MaxSize 10", params.Size)
	}
}

func TestParseListParamsPageAndSize(t *testing.T) {
	q := url.Values{"page": {"3"}, "size": {"15"}}

	params, err := ParseListParams(q, ListParseConfig{})
	if err != nil {
		t.Fatalf("ParseListParams() error = %v, want nil", err)
	}
	if params.Page != 3 || params.Size != 15 {
		t.Errorf("Page/Size = %d/%d, want 3/15", params.Page, params.Size)
	}
}

func TestParseListParamsInvalidPageAndSize(t *testing.T) {
	tests := []struct {
		name string
		q    url.Values
	}{
		{name: "non-numeric page", q: url.Values{"page": {"abc"}}},
		{name: "zero page", q: url.Values{"page": {"0"}}},
		{name: "negative page", q: url.Values{"page": {"-1"}}},
		{name: "non-numeric size", q: url.Values{"size": {"abc"}}},
		{name: "zero size", q: url.Values{"size": {"0"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseListParams(tt.q, ListParseConfig{}); err == nil {
				t.Errorf("ParseListParams(%v) error = nil, want error", tt.q)
			}
		})
	}
}

func TestParseListParamsSort(t *testing.T) {
	cfg := ListParseConfig{AllowedSortFields: []string{"name", "id"}}
	q := url.Values{"sort": {"name,ASC", "id,desc"}}

	params, err := ParseListParams(q, cfg)
	if err != nil {
		t.Fatalf("ParseListParams() error = %v, want nil", err)
	}
	want := []common.SortSpec{
		{Field: "name", Direction: common.SortAsc},
		{Field: "id", Direction: common.SortDesc},
	}
	if len(params.Sorts) != len(want) {
		t.Fatalf("Sorts = %v, want %v", params.Sorts, want)
	}
	for i, s := range want {
		if params.Sorts[i] != s {
			t.Errorf("Sorts[%d] = %v, want %v", i, params.Sorts[i], s)
		}
	}
}

func TestParseListParamsSortRejectsDisallowedField(t *testing.T) {
	cfg := ListParseConfig{AllowedSortFields: []string{"name"}}
	q := url.Values{"sort": {"secret,ASC"}}

	if _, err := ParseListParams(q, cfg); err == nil {
		t.Error("ParseListParams() error = nil, want error for disallowed sort field")
	}
}

func TestParseListParamsSortRejectsMalformedValue(t *testing.T) {
	tests := []struct {
		name string
		sort string
	}{
		{name: "missing direction", sort: "name"},
		{name: "invalid direction", sort: "name,SIDEWAYS"},
	}

	cfg := ListParseConfig{AllowedSortFields: []string{"name"}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := url.Values{"sort": {tt.sort}}
			if _, err := ParseListParams(q, cfg); err == nil {
				t.Errorf("ParseListParams(sort=%q) error = nil, want error", tt.sort)
			}
		})
	}
}

func TestParseListParamsFilters(t *testing.T) {
	cfg := ListParseConfig{
		AllowedSortFields:   []string{"name"},
		AllowedFilterFields: []string{"name", "source"},
	}
	q := url.Values{
		"name":   {"Event1"},
		"source": {"app"},
		"other":  {"ignored"},
		"page":   {"1"},
		"size":   {"20"},
		"sort":   {"name,ASC"},
	}

	params, err := ParseListParams(q, cfg)
	if err != nil {
		t.Fatalf("ParseListParams() error = %v, want nil", err)
	}
	want := map[string]string{"name": "Event1", "source": "app"}
	if len(params.Filters) != len(want) {
		t.Fatalf("filters = %v, want %v", params.Filters, want)
	}
	for k, v := range want {
		if params.Filters[k] != v {
			t.Errorf("filters[%q] = %q, want %q", k, params.Filters[k], v)
		}
	}
}
