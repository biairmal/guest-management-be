package handler

import (
	"net/http"
	"strconv"

	"github.com/biairmal/go-sdk/repository"
)

func parseLimitOffset(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 100 {
				limit = 100
			}
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}

func parseSort(r *http.Request) repository.Sort {
	field := r.URL.Query().Get("sort_field")
	if field == "" {
		field = "created_at"
	}
	dir := repository.SortAsc
	if r.URL.Query().Get("sort_dir") == "desc" {
		dir = repository.SortDesc
	}
	return repository.Sort{Field: field, Direction: dir}
}
