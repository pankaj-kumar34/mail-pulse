package main

import (
	"sort"
	"sync"
)

// ──────────────────────────────────────────────
// Log Store — append-only in-memory log store
// ──────────────────────────────────────────────

type logStore struct {
	mu        sync.RWMutex
	logs      []ParsedLogEntry
	seenIDs   map[string]struct{}
	maxTs     int64 // cached max timestamp — updated on every add(), avoids O(n) scan

	// NDR/DSN queue IDs — populated when qmgr sees from=<>.
	// All entries for these queue IDs are excluded from every metric.
	ndrQueues map[string]struct{}

	// queueFrom maps queueID → sender address, built from qmgr/pickup lines.
	// Delivery lines (smtp/lmtp) don't carry from=; we look it up here.
	queueFrom map[string]string
}

func newLogStore() *logStore {
	return &logStore{
		seenIDs:   make(map[string]struct{}),
		ndrQueues: make(map[string]struct{}),
		queueFrom: make(map[string]string),
	}
}

// add deduplicates and appends e. Tracks NDR queue IDs and queue→sender map.
func (s *logStore) add(e *ParsedLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, seen := s.seenIDs[e.ID]; seen {
		return
	}
	s.seenIDs[e.ID] = struct{}{}
	s.logs = append(s.logs, *e)

	// Keep maxTs up to date so callers pay O(1) instead of O(n).
	if e.Timestamp > s.maxTs {
		s.maxTs = e.Timestamp
	}

	// Track NDR/DSN queue IDs so all metric queries can exclude them in one place.
	if e.From == "<>" && e.QueueID != "" {
		s.ndrQueues[e.QueueID] = struct{}{}
	}

	// Record sender for this queue ID (from qmgr/pickup lines that carry from=).
	if e.QueueID != "" && e.From != "" && e.From != "<>" {
		if _, exists := s.queueFrom[e.QueueID]; !exists {
			s.queueFrom[e.QueueID] = e.From
		}
	}
}

// clear resets the store to empty.
func (s *logStore) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.logs = nil
	s.maxTs = 0
	s.seenIDs = make(map[string]struct{})
	s.ndrQueues = make(map[string]struct{})
	s.queueFrom = make(map[string]string)
}

// isNDR returns true if the entry is an NDR/DSN and should be excluded from metrics.
func (s *logStore) isNDR(e *ParsedLogEntry) bool {
	if e.From == "<>" {
		return true
	}
	if e.QueueID != "" {
		_, ok := s.ndrQueues[e.QueueID]
		return ok
	}
	return false
}

// maxTimestamp returns the latest entry timestamp (unix ms). O(1) — maintained by add().
func (s *logStore) maxTimestamp() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.maxTs
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func avgSlice(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func sortedTimeSeries(m map[int64][]float64) []TimeSeriesPoint {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	pts := make([]TimeSeriesPoint, 0, len(keys))
	for _, k := range keys {
		pts = append(pts, TimeSeriesPoint{Timestamp: k * 60000, Value: avgSlice(m[k])})
	}
	return pts
}

func sortedCountSeries(m map[int64]int) []TimeSeriesPoint {
	if len(m) == 0 {
		return nil
	}
	// Find min/max 5-minute buckets
	var minK, maxK int64
	first := true
	for k := range m {
		if first || k < minK { minK = k }
		if first || k > maxK { maxK = k }
		first = false
	}
	// Emit every 5-minute bucket in range, filling missing buckets with 0
	// so the chart correctly drops to zero between activity bursts
	pts := make([]TimeSeriesPoint, 0, int(maxK-minK+1))
	for k := minK; k <= maxK; k++ {
		pts = append(pts, TimeSeriesPoint{Timestamp: k * 300000, Value: float64(m[k])})
	}
	return pts
}

func topN(m map[string]int, n int) []DomainCount {
	type kv struct {
		k string
		v int
	}
	pairs := make([]kv, 0, len(m))
	for k, v := range m {
		pairs = append(pairs, kv{k, v})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].v > pairs[j].v })
	if len(pairs) > n {
		pairs = pairs[:n]
	}
	out := make([]DomainCount, len(pairs))
	for i, p := range pairs {
		out[i] = DomainCount{Domain: p.k, Count: p.v}
	}
	return out
}
