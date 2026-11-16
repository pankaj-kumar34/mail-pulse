package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// ──────────────────────────────────────────────
// Mail Queue Service — fetches live queue from SMTP relay API
// ──────────────────────────────────────────────

type MailQueueService struct {
	apiBase string
}

func NewMailQueueService() *MailQueueService {
	base := os.Getenv("MAIL_QUEUE_API")
	if base == "" {
		base = "http://localhost:3044/smtp-relay"
	}
	return &MailQueueService{apiBase: base}
}

// fetchMailq issues the HTTP request and returns the response body.
// The caller is responsible for closing the body.
// Separated from parsing so it can be replaced in tests with any io.ReadCloser.
func (m *MailQueueService) fetchMailq(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.apiBase+"/mailq", nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return resp.Body, nil
}

// parseMailqStream reads the NDJSON stream from the relay API.
// The format is one line of "pod <id>" followed by newline-delimited JSON objects.
// Uses json.Decoder directly on each line to avoid the Unmarshal([]byte) round-trip.
func parseMailqStream(r io.Reader) (podID string, entries []MailQueueEntry, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		// Trim whitespace in-place without allocating a new string
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "pod ") {
			podID = strings.TrimPrefix(trimmed, "pod ")
			continue
		}
		var e MailQueueEntry
		if err := json.Unmarshal(line, &e); err == nil {
			entries = append(entries, e)
		}
	}
	if err := scanner.Err(); err != nil {
		return podID, entries, fmt.Errorf("scan mailq stream: %w", err)
	}
	return podID, entries, nil
}

// matchesQuery reports whether e contains the pre-lowercased query in any
// searchable field. Centralises all strings.ToLower calls and field checks.
func matchesQuery(e MailQueueEntry, lower string) bool {
	if strings.Contains(strings.ToLower(e.QueueID), lower) ||
		strings.Contains(strings.ToLower(e.Sender), lower) ||
		strings.Contains(strings.ToLower(e.QueueName), lower) {
		return true
	}
	for _, r := range e.Recipients {
		if strings.Contains(strings.ToLower(r.Address), lower) {
			return true
		}
	}
	return false
}

func (m *MailQueueService) GetQueue(page, limit int, search, queue, sortBy, sortOrder string) PaginatedQueue {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := m.fetchMailq(ctx)
	if err != nil {
		return PaginatedQueue{}
	}
	defer body.Close()

	podID, entries, err := parseMailqStream(body)
	if err != nil {
		return PaginatedQueue{}
	}

	// Filter — preallocated to capacity of entries; single append per entry
	searchLower := strings.ToLower(search)
	filtered := make([]MailQueueEntry, 0, len(entries))
	for _, e := range entries {
		if queue != "" && e.QueueName != queue {
			continue
		}
		if searchLower != "" && !matchesQuery(e, searchLower) {
			continue
		}
		filtered = append(filtered, e)
	}

	// Sort
	sort.SliceStable(filtered, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "queue_id":     less = filtered[i].QueueID < filtered[j].QueueID
		case "queue_name":   less = filtered[i].QueueName < filtered[j].QueueName
		case "message_size": less = filtered[i].MessageSize < filtered[j].MessageSize
		case "sender":       less = filtered[i].Sender < filtered[j].Sender
		default:             less = filtered[i].ArrivalTime < filtered[j].ArrivalTime
		}
		if sortOrder == "desc" { return !less }
		return less
	})

	total := len(filtered)
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 { totalPages = 1 }
	if page < 1 { page = 1 }
	if page > totalPages { page = totalPages }
	start := (page - 1) * limit
	end := start + limit
	if end > total { end = total }

	return PaginatedQueue{
		Entries: filtered[start:end],
		PodID:   podID,
		Pagination: PaginationInfo{
			Page: page, Limit: limit, Total: total, TotalPages: totalPages,
			HasNext: page < totalPages, HasPrev: page > 1,
		},
	}
}
