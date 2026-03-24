package dashboardapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func ReadLimit(r *http.Request, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return fallback
	}
	return limit
}

func ReadOffset(r *http.Request, key string) int {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return 0
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}
	return value
}

func ReadSort(r *http.Request) string {
	sort := strings.TrimSpace(r.URL.Query().Get("sort"))
	if strings.EqualFold(sort, "asc") {
		return "asc"
	}
	return "desc"
}

func RespondJSON(w http.ResponseWriter, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func PageSummary(total, offset, limit, count int) string {
	if total == 0 {
		return "Showing 0-0 of 0 · page 0 of 0"
	}
	start := offset + 1
	end := offset + count
	pageNumber := (offset / limit) + 1
	totalPages := (total + limit - 1) / limit
	return fmt.Sprintf("Showing %d-%d of %d · page %d of %d", start, end, total, pageNumber, totalPages)
}
