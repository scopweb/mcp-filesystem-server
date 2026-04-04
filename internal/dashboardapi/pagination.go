package dashboardapi

import (
	"strings"

	"github.com/scopweb/mcp-filesystem-go-light/internal/logview"
)

type AuditPage struct {
	Items  []logview.AuditEntry `json:"items"`
	Total  int                  `json:"total"`
	Offset int                  `json:"offset"`
	Limit  int                  `json:"limit"`
	Sort   string               `json:"sort"`
}

type ProxyPage struct {
	Items  []logview.ProxyEntry `json:"items"`
	Total  int                  `json:"total"`
	Offset int                  `json:"offset"`
	Limit  int                  `json:"limit"`
	Sort   string               `json:"sort"`
}

type RequestPage struct {
	RequestID string    `json:"request_id"`
	Server    AuditPage `json:"server"`
	Proxy     ProxyPage `json:"proxy"`
}

func BuildAuditPage(entries []logview.AuditEntry, sortOrder string, offset, limit int) AuditPage {
	total := len(entries)
	items := applyAuditPagination(entries, sortOrder, offset, limit)
	return AuditPage{
		Items:  items,
		Total:  total,
		Offset: maxInt(offset, 0),
		Limit:  limit,
		Sort:   normalizeSort(sortOrder),
	}
}

func BuildProxyPage(entries []logview.ProxyEntry, sortOrder string, offset, limit int) ProxyPage {
	total := len(entries)
	items := applyProxyPagination(entries, sortOrder, offset, limit)
	return ProxyPage{
		Items:  items,
		Total:  total,
		Offset: maxInt(offset, 0),
		Limit:  limit,
		Sort:   normalizeSort(sortOrder),
	}
}

func BuildRequestPage(requestID string, serverEntries []logview.AuditEntry, proxyEntries []logview.ProxyEntry, sortOrder string, offset, limit int) RequestPage {
	return RequestPage{
		RequestID: requestID,
		Server:    BuildAuditPage(serverEntries, sortOrder, offset, limit),
		Proxy:     BuildProxyPage(proxyEntries, sortOrder, offset, limit),
	}
}

func FilterErrorEntries(entries []logview.AuditEntry) []logview.AuditEntry {
	errorsOnly := make([]logview.AuditEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.Status == "error" {
			errorsOnly = append(errorsOnly, entry)
		}
	}
	return errorsOnly
}

func FilterRequestEntries(entries []logview.AuditEntry, requestID string) []logview.AuditEntry {
	matched := make([]logview.AuditEntry, 0)
	for _, entry := range entries {
		if entry.RequestID == requestID {
			matched = append(matched, entry)
		}
	}
	return matched
}

func FilterProxyRequestEntries(entries []logview.ProxyEntry, requestID string) []logview.ProxyEntry {
	matched := make([]logview.ProxyEntry, 0)
	for _, entry := range entries {
		if entry.RequestID == requestID {
			matched = append(matched, entry)
		}
	}
	return matched
}

func applyAuditPagination(entries []logview.AuditEntry, sortOrder string, offset, limit int) []logview.AuditEntry {
	if strings.EqualFold(sortOrder, "asc") {
		reversed := make([]logview.AuditEntry, len(entries))
		for i := range entries {
			reversed[i] = entries[len(entries)-1-i]
		}
		entries = reversed
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(entries) {
		return []logview.AuditEntry{}
	}
	entries = entries[offset:]
	if limit < len(entries) {
		return entries[:limit]
	}
	return entries
}

func applyProxyPagination(entries []logview.ProxyEntry, sortOrder string, offset, limit int) []logview.ProxyEntry {
	if strings.EqualFold(sortOrder, "asc") {
		reversed := make([]logview.ProxyEntry, len(entries))
		for i := range entries {
			reversed[i] = entries[len(entries)-1-i]
		}
		entries = reversed
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(entries) {
		return []logview.ProxyEntry{}
	}
	entries = entries[offset:]
	if limit < len(entries) {
		return entries[:limit]
	}
	return entries
}

func normalizeSort(sortOrder string) string {
	if strings.EqualFold(sortOrder, "asc") {
		return "asc"
	}
	return "desc"
}

func maxInt(value, minimum int) int {
	if value < minimum {
		return minimum
	}
	return value
}
