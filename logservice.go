package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ──────────────────────────────────────────────
// Line Index — byte-offset index for fast random access to large log files
// ──────────────────────────────────────────────

type lineIndex struct {
	mu       sync.RWMutex
	filename string
	offsets  []int64 // byte offset of each line start
	fileSize int64   // file size at last index build
}

func (idx *lineIndex) build(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.offsets = idx.offsets[:0]
	idx.fileSize = info.Size()

	var offset int64
	buf := bufio.NewReaderSize(f, 256*1024)
	if info.Size() > 0 {
		idx.offsets = append(idx.offsets, 0) // first line starts at 0
	}
	for {
		line, err := buf.ReadBytes('\n')
		offset += int64(len(line))
		if err != nil {
			break
		}
		// offset == size means the file ended with '\n' — no line starts there
		if offset < info.Size() {
			idx.offsets = append(idx.offsets, offset)
		}
	}
	return nil
}

// extend appends any new lines written since last index build.
// Returns reset=true when the file shrank (rotated/truncated) and the index
// was rebuilt from scratch — callers must re-fetch instead of appending.
func (idx *lineIndex) extend(path string) (int, bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, false, err
	}

	idx.mu.RLock()
	currentSize := idx.fileSize
	idx.mu.RUnlock()

	if info.Size() == currentSize {
		return 0, false, nil
	}
	if info.Size() < currentSize {
		// Rotated/truncated — offsets all point past EOF now. Rebuild.
		if err := idx.build(path); err != nil {
			return 0, true, err
		}
		return 0, true, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, false, err
	}
	defer f.Close()

	// A line starts at currentSize only if the previous content ended with '\n';
	// otherwise that byte is mid-line and its start is already indexed.
	prevEndsLine := true
	if currentSize > 0 {
		var last [1]byte
		if _, err := f.ReadAt(last[:], currentSize-1); err == nil {
			prevEndsLine = last[0] == '\n'
		}
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	added := 0
	offset := currentSize
	if prevEndsLine {
		idx.offsets = append(idx.offsets, offset)
		added++
	}
	buf := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := buf.ReadBytes('\n')
		offset += int64(len(line))
		if len(line) > 0 && err == nil && offset < info.Size() {
			idx.offsets = append(idx.offsets, offset)
			added++
		}
		if err != nil {
			break
		}
	}
	idx.fileSize = info.Size()
	return added, false, nil
}

func (idx *lineIndex) totalLines() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.offsets)
}

// ──────────────────────────────────────────────
// Ingest State — tracks a running async ingest
// ──────────────────────────────────────────────

type ingestState struct {
	mu      sync.Mutex
	running bool
	pct     int
	msg     string
	done    bool
	err     string
	gen     int // generation: goroutine aborts when its gen != current
}

func (s *ingestState) start() (gen int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gen++
	s.running = true
	s.pct = 0
	s.msg = "Starting…"
	s.done = false
	s.err = ""
	return s.gen
}

func (s *ingestState) update(gen, pct int, msg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return false // superseded — goroutine should stop
	}
	s.pct = pct
	s.msg = msg
	return true
}

func (s *ingestState) finish(gen int, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.gen != gen {
		return
	}
	s.running = false
	s.done = true
	s.err = errMsg
	s.pct = 100
	if errMsg != "" {
		s.msg = "Error"
	} else {
		s.msg = "Done"
	}
}

func (s *ingestState) snapshot() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]interface{}{
		"running": s.running,
		"pct":     s.pct,
		"msg":     s.msg,
		"done":    s.done,
		"error":   s.err,
	}
}

// ──────────────────────────────────────────────
// Log Service — Wails-exported API
// ──────────────────────────────────────────────

type LogService struct {
	store        *logStore
	mu           sync.Mutex
	selectedFile string
	ingested     bool
	logsDir      string
	lineIdx      lineIndex
	istate       ingestState
}

func findLogsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mail-pulse", "logs")
}

var logDateRe = regexp.MustCompile(`(\d{1,2})-(\d{1,2})-(\d{4})\.log$`)

// ──────────────────────────────────────────────
// Prefs — remembers the last file the user picked.
// If the remembered file no longer exists, falls back to the newest log,
// so a stale pref can never cause an error on startup.
// ──────────────────────────────────────────────

func prefsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mail-pulse", "prefs.json")
}

func loadSelectedFile(logsDir string) string {
	data, err := os.ReadFile(prefsPath())
	if err == nil {
		var prefs struct {
			SelectedFile string `json:"selected_file"`
		}
		if json.Unmarshal(data, &prefs) == nil && prefs.SelectedFile != "" {
			if _, err := os.Stat(filepath.Join(logsDir, prefs.SelectedFile)); err == nil {
				return prefs.SelectedFile
			}
		}
	}
	return newestLogFile(logsDir)
}

func saveSelectedFile(file string) {
	data, _ := json.Marshal(map[string]string{"selected_file": file})
	_ = os.WriteFile(prefsPath(), data, 0644)
}

// logDateKey extracts a sortable key (yyyymmdd) from log filenames that end in
// a date like "-16-11-2026.log" (day-first; swapped when the month is > 12),
// so "newest" follows the log's date rather than when the file was copied in.
// Returns 0 when the filename carries no date.
func logDateKey(name string) int64 {
	m := logDateRe.FindStringSubmatch(name)
	if m == nil {
		return 0
	}
	d, _ := strconv.Atoi(m[1])
	mo, _ := strconv.Atoi(m[2])
	y, _ := strconv.Atoi(m[3])
	if d <= 12 && mo > 12 {
		d, mo = mo, d
	}
	return int64(y)*10000 + int64(mo)*100 + int64(d)
}

// newestLogFile returns the newest .log file in dir — by the date in the
// filename when present, otherwise by modification time. Falls back to
// "mail.0.log" when the directory has no log files at all.
func newestLogFile(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "mail.0.log"
	}
	var best string
	var bestKey, bestMod int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		var mod int64
		if info, err := e.Info(); err == nil {
			mod = info.ModTime().UnixNano()
		}
		key := logDateKey(e.Name())
		if best == "" || key > bestKey || (key == bestKey && mod > bestMod) {
			best, bestKey, bestMod = e.Name(), key, mod
		}
	}
	if best == "" {
		return "mail.0.log"
	}
	return best
}

func NewLogService() *LogService {
	logsDir := findLogsDir()
	return &LogService{
		store:        newLogStore(),
		selectedFile: loadSelectedFile(logsDir),
		logsDir:      logsDir,
	}
}

func (l *LogService) SetLogsDir(dir string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logsDir = dir
}

func (l *LogService) GetStatus() map[string]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return map[string]string{
		"selectedFile": l.selectedFile,
		"logsDir":      l.logsDir,
	}
}

func (l *LogService) ListLogFiles() []string {
	l.mu.Lock()
	dir := l.logsDir
	l.mu.Unlock()

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".log") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	return files
}

// runIngest is the synchronous parse loop used by the goroutine.
// It honours the generation counter to abort early if superseded.
func (l *LogService) runIngest(gen int) error {
	l.mu.Lock()
	filePath := filepath.Join(l.logsDir, l.selectedFile)
	l.mu.Unlock()

	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file_not_found")
	}
	fileSize := info.Size()

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("file_not_found")
	}
	defer f.Close()

	emit := func(pct int, msg string) {
		if app := application.Get(); app != nil {
			app.Event.Emit("ingest:progress", map[string]interface{}{"pct": pct, "msg": msg})
		}
	}

	emit(0, "Starting…")

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	var bytesRead int64
	lastPct := 0
	var prevMonth int
	var yearOffset int
	for scanner.Scan() {
		raw := scanner.Bytes()
		bytesRead += int64(len(raw)) + 1
		line := strings.TrimSpace(string(raw))
		if line != "" {
			if e := parsePostfixLogWithState(line, &prevMonth, &yearOffset); e != nil {
				if !l.addIfCurrent(gen, e) {
					return fmt.Errorf("superseded") // another ingest started
				}
			}
		}
		if fileSize > 0 {
			pct := int(bytesRead * 100 / fileSize)
			if pct != lastPct && pct%5 == 0 {
				if !l.istate.update(gen, pct, "Parsing…") {
					return fmt.Errorf("superseded") // another ingest started
				}
				emit(pct, "Parsing…")
				lastPct = pct
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan_error: %w", err)
	}
	return nil
}

// StartIngest kicks off an async ingest goroutine and returns immediately.
// If an ingest is already running it is cancelled (superseded) and a new one starts.
// Returns {"started": true, "selectedFile": <name>}.
func (l *LogService) StartIngest() map[string]interface{} {
	l.mu.Lock()
	l.ingested = false
	sel := l.selectedFile
	l.mu.Unlock()

	gen := l.istate.start()

	go func() {
		err := l.runIngest(gen)
		if err != nil && err.Error() == "superseded" {
			return // silently exit — a newer generation took over
		}
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		} else {
			l.mu.Lock()
			l.ingested = true
			l.mu.Unlock()
			// Warm the query cache for the summary windows so GetQueueSummaries is O(1).
			for _, hours := range []int{0, 1, 6, 24} {
				l.store.getQueueSummaries(l.since(hours), "")
			}
		}
		l.istate.finish(gen, errMsg)
		if app := application.Get(); app != nil {
			app.Event.Emit("ingest:progress", map[string]interface{}{"pct": 100, "msg": map[bool]string{true: "Error", false: "Done"}[errMsg != ""]})
		}
	}()

	return map[string]interface{}{"started": true, "selectedFile": sel}
}

// GetIngestStatus returns a snapshot of the current ingest state.
func (l *LogService) GetIngestStatus() map[string]interface{} {
	return l.istate.snapshot()
}

// IngestFile is a synchronous ingest used internally (e.g. by COSService).
// It blocks until parsing is complete and returns the legacy result map.
func (l *LogService) IngestFile() map[string]interface{} {
	l.mu.Lock()
	if l.ingested {
		sel := l.selectedFile
		l.mu.Unlock()
		return map[string]interface{}{"ingested": false, "status": "already_ingested", "selectedFile": sel}
	}
	sel := l.selectedFile
	l.mu.Unlock()

	gen := l.istate.start()
	err := l.runIngest(gen)
	if err != nil {
		errMsg := err.Error()
		l.istate.finish(gen, errMsg)
		return map[string]interface{}{"ingested": false, "status": "error", "error": errMsg}
	}
	l.istate.finish(gen, "")
	l.mu.Lock()
	l.ingested = true
	l.mu.Unlock()
	return map[string]interface{}{"ingested": true, "status": "complete", "selectedFile": sel}
}

// abortAndClear bumps the ingest generation and empties the store under one lock
// (istate.mu → store.mu) so a superseded ingest can never add stale lines to the
// freshly cleared store — its check+add pair in addIfCurrent shares this ordering.
func (l *LogService) abortAndClear() {
	l.istate.mu.Lock()
	defer l.istate.mu.Unlock()
	l.store.mu.Lock()
	defer l.store.mu.Unlock()
	l.istate.gen++
	l.store.clearLocked()
}

// addIfCurrent adds e only while gen is still the live ingest generation.
func (l *LogService) addIfCurrent(gen int, e *ParsedLogEntry) bool {
	l.istate.mu.Lock()
	defer l.istate.mu.Unlock()
	if l.istate.gen != gen {
		return false
	}
	l.store.mu.Lock()
	l.store.addLocked(e)
	l.store.mu.Unlock()
	return true
}

// ResetForFile switches the selected file and clears the store so a fresh
// ingest can start. Call StartIngest() afterwards.
func (l *LogService) ResetForFile(filename string) {
	l.mu.Lock()
	if filename != "" {
		l.selectedFile = filename
		saveSelectedFile(filename)
	}
	l.ingested = false
	l.mu.Unlock()
	l.abortAndClear()
}

// RefreshFile resets the selected file and starts an async ingest.
// Returns immediately with {"started": true, "selectedFile": <name>}.
func (l *LogService) RefreshFile(filename string) map[string]interface{} {
	l.ResetForFile(filename)
	return l.StartIngest()
}

// ── Metrics & Insights ────────────────────────────────────────────────────────

// since converts a sinceHours window (0 = all time) to an epoch-ms cutoff.
func (l *LogService) since(sinceHours int) int64 {
	if sinceHours <= 0 {
		return 0
	}
	return l.store.maxTimestamp() - int64(sinceHours)*3600000
}

func (l *LogService) GetMetrics(sinceHours int) Metrics {
	since := l.since(sinceHours)
	return l.store.getMetrics(since)
}

func (l *LogService) GetVolumeInsights(sinceHours int) VolumeInsights {
	since := l.since(sinceHours)
	return l.store.getVolumeInsights(since)
}

func (l *LogService) GetDelayInsights(sinceHours int) DelayInsights {
	since := l.since(sinceHours)
	return l.store.getDelayInsights(since)
}

// GetLogTimeRange returns the earliest and latest timestamps (unix ms) in the loaded log.
// O(1) — min/max are maintained on add().
func (l *LogService) GetLogTimeRange() map[string]int64 {
	minTs, maxTs := l.store.timeRange()
	return map[string]int64{"min": minTs, "max": maxTs}
}

// ── Data Access ───────────────────────────────────────────────────────────────

func (l *LogService) GetMessages(sinceHours, page, limit int, search, status, sortBy, sortOrder string) PaginatedMessages {
	since := l.since(sinceHours)
	return l.store.getMessages(since, page, limit, search, status, sortBy, sortOrder)
}

func (l *LogService) GetErrors(sinceHours, page, limit int, errType, sortOrder string) PaginatedLogs {
	since := l.since(sinceHours)
	return l.store.getErrorsPaginated(since, page, limit, errType, sortOrder)
}

func (l *LogService) GetRecipientDrilldown(domain string, sinceHours int) RecipientDrilldown {
	since := l.since(sinceHours)
	return l.store.getRecipientDrilldown(domain, since)
}

func (l *LogService) GetQueueAgeDistribution(sinceHours int) QueueAgeDistribution {
	since := l.since(sinceHours)
	return l.store.getQueueAgeDistribution(since)
}

func (l *LogService) GetRelayInsights(sinceHours int) RelayInsights {
	since := l.since(sinceHours)
	return l.store.getRelayInsights(since)
}

func (l *LogService) GetRelayMessages(relay, status string, sinceHours int) []RelayMessage {
	since := l.since(sinceHours)
	return l.store.getRelayMessages(relay, status, since)
}

func (l *LogService) GetGarbage(sinceHours int) []GarbageRow {
	since := l.since(sinceHours)
	return l.store.getGarbage(since)
}

func (l *LogService) GetSenderStats(sinceHours int) []SenderStat {
	since := l.since(sinceHours)
	return l.store.getSenderStats(since)
}

func (l *LogService) GetMessagesByReason(reason, status string, sinceHours int) []ReasonMessage {
	since := l.since(sinceHours)
	return l.store.getMessagesByReason(reason, status, since)
}

func (l *LogService) GetMessagesBySender(from, status string, sinceHours int) []ReasonMessage {
	since := l.since(sinceHours)
	return l.store.getMessagesBySender(from, status, since)
}

func (l *LogService) GetIPReputationReport(sinceHours int) IPReputationReport {
	since := l.since(sinceHours)
	return l.store.getIPReputationReport(since)
}

// GetBlockEventsSince returns block events for a single (IP, service) pair whose
// timestamp is >= sinceMs (epoch milliseconds). Use the SinceMs value returned by
// GetIPReputationReport so the drilldown uses exactly the same time window as the card.
func (l *LogService) GetBlockEventsSince(ourIP, service string, sinceMs int64) []BlockEvent {
	return l.store.getBlockEvents(ourIP, service, sinceMs)
}


// IndexFile builds the line index for a file. Returns total line count.
func (l *LogService) IndexFile(filename string) IndexFileResult {
	if filename == "" {
		filename = l.selectedFile
	}
	path := filepath.Join(l.logsDir, filename)
	l.lineIdx.filename = filename
	if err := l.lineIdx.build(path); err != nil {
		return IndexFileResult{Error: err.Error()}
	}
	return IndexFileResult{Total: l.lineIdx.totalLines()}
}

// GetLines returns lines [start, start+count) from the indexed file (0-based start).
// Uses a single lock acquisition and a buffered scanner to minimise allocations.
func (l *LogService) GetLines(filename string, start, count int) GetLinesResult {
	if filename == "" {
		filename = l.selectedFile
	}
	if l.lineIdx.filename != filename {
		l.IndexFile(filename)
	}

	// Single lock: read total, startOffset, and fileSize together.
	l.lineIdx.mu.RLock()
	total := len(l.lineIdx.offsets)
	var startOffset int64
	if total > 0 && start < total {
		startOffset = l.lineIdx.offsets[start]
	}
	l.lineIdx.mu.RUnlock()

	if total == 0 || start >= total {
		return GetLinesResult{Lines: []string{}, Total: total}
	}

	end := start + count
	if end > total { end = total }
	want := end - start

	f, err := os.Open(filepath.Join(l.logsDir, filename))
	if err != nil {
		return GetLinesResult{Lines: []string{}, Total: total}
	}
	defer f.Close()

	if _, err := f.Seek(startOffset, io.SeekStart); err != nil {
		return GetLinesResult{Lines: []string{}, Total: total}
	}

	// Buffered scanner reads exactly `want` lines — no bulk ReadAt + Split.
	lines := make([]string, 0, want)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)
	for len(lines) < want && scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return GetLinesResult{Lines: []string{}, Total: total}
	}
	return GetLinesResult{Lines: lines, Total: total}
}

// SearchLines searches the file for query (case-insensitive), paginated by matchOffset/limit.
func (l *LogService) SearchLines(filename, query string, matchOffset, limit int) SearchLinesResult {
	if filename == "" {
		filename = l.selectedFile
	}
	if query == "" {
		return SearchLinesResult{Matches: []SearchMatch{}}
	}
	if l.lineIdx.filename != filename {
		l.IndexFile(filename)
	}

	f, err := os.Open(filepath.Join(l.logsDir, filename))
	if err != nil {
		return SearchLinesResult{Matches: []SearchMatch{}}
	}
	defer f.Close()

	lower := strings.ToLower(query)
	var matches []SearchMatch
	total := 0

	reader := bufio.NewReaderSize(f, 512*1024)
	lineNum := 0
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			text := strings.TrimRight(line, "\n\r")
			if strings.Contains(strings.ToLower(text), lower) {
				if total >= matchOffset && total < matchOffset+limit {
					matches = append(matches, SearchMatch{LineNumber: lineNum, Text: text})
				}
				total++
			}
			lineNum++
		}
		if err != nil { break }
	}

	if matches == nil { matches = []SearchMatch{} }
	return SearchLinesResult{Matches: matches, Total: total}
}

// PollNewLines checks for new lines appended since last index and returns them.
// reset=true means the file shrank (rotation) and was re-indexed — clear state and refetch.
func (l *LogService) PollNewLines(filename string) PollNewLinesResult {
	if filename == "" {
		filename = l.selectedFile
	}
	if l.lineIdx.filename != filename {
		l.IndexFile(filename)
		return PollNewLinesResult{Lines: []string{}, Total: l.lineIdx.totalLines()}
	}

	path := filepath.Join(l.logsDir, filename)
	prevTotal := l.lineIdx.totalLines()

	added, reset, err := l.lineIdx.extend(path)
	if err != nil {
		return PollNewLinesResult{Lines: []string{}, Total: prevTotal}
	}
	if reset {
		return PollNewLinesResult{Lines: []string{}, Total: l.lineIdx.totalLines(), Reset: true}
	}
	if added == 0 {
		return PollNewLinesResult{Lines: []string{}, Total: prevTotal}
	}

	result := l.GetLines(filename, prevTotal, added)
	return PollNewLinesResult{Lines: result.Lines, Total: result.Total, NewCount: added}
}

func (l *LogService) GetHeatmap(sinceHours int) []HeatmapCell {
	since := l.since(sinceHours)
	return l.store.getHeatmap(since)
}

// GetJourney resolves a queue ID (or recipient/sender substring) and returns
// the full event timeline for every matching queue — the message journey view.
func (l *LogService) GetJourney(query string) JourneyResult {
	return l.store.getJourney(query)
}

// GetMessagesByBucket returns all deferred/bounced attempts in a taxonomy bucket
// ("Block / reputation", "Hard bounce", "Soft / temporary", "Connection / network", "Other").
func (l *LogService) GetMessagesByBucket(bucket, status string, sinceHours int) []ReasonMessage {
	since := l.since(sinceHours)
	return l.store.getMessagesByBucket(bucket, status, since)
}

func (l *LogService) GetTopDeferredQueues(sinceHours, limit int) []string {
	since := l.since(sinceHours)
	return l.store.getTopDeferredQueues(since, limit)
}

func (l *LogService) GetQueueSummaries(sinceHours, page, limit int, filterFrom, statusFilter string) PaginatedQueueSummaries {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}

	// Pull full slice — cached (O(1) when pre-warmed at ingest-end) or computed on demand.
	all := l.store.getQueueSummaries(l.since(sinceHours), filterFrom)

	// Count unknowns across the full dataset (before status filter) so the
	// frontend can show a rotation banner without needing to scan all pages.
	unknownCount := 0
	for i := range all {
		if all[i].FinalStatus == "unknown" {
			unknownCount++
		}
	}

	// Apply status filter server-side so pagination is correct per-filter.
	// Copy, never compact in place — `all` is the shared cached slice.
	if statusFilter != "" {
		filtered := make([]QueueSummary, 0, len(all))
		for i := range all {
			if all[i].FinalStatus == statusFilter {
				filtered = append(filtered, all[i])
			}
		}
		all = filtered
	}

	total := len(all)
	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}

	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	return PaginatedQueueSummaries{
		Rows:         all[start:end],
		UnknownCount: unknownCount,
		Pagination: PaginationInfo{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
			HasNext:    page < totalPages,
			HasPrev:    page > 1,
		},
	}
}

// ──────────────────────────────────────────────
// warn_if_reject monitor
// ──────────────────────────────────────────────

// GetWarnInsights returns RCPT-stage policy data — warn_if_reject downgrades
// plus hard NOQUEUE rejects — for the warn→enforce flip decision.
func (l *LogService) GetWarnInsights(sinceHours int) WarnInsights {
	since := l.since(sinceHours)
	return l.store.getWarnInsights(since)
}

// CheckWarnDomains runs live DNS (MX then A) lookups for the given warned
// domains. Status: nxdomain = still-dead garbage (safe to enforce),
// a-only = no MX but A fallback exists (postfix would still deliver),
// ok = live MX (transient warning or recovered — flip risk),
// tempfail = resolver trouble, inconclusive.
func (l *LogService) CheckWarnDomains(domains []string) []DomainDNSStatus {
	out := make([]DomainDNSStatus, 0, len(domains))
	for _, dom := range domains {
		out = append(out, checkDomainDNS(dom))
	}
	return out
}

func checkDomainDNS(domain string) DomainDNSStatus {
	st := DomainDNSStatus{Domain: domain}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	mx, err := net.DefaultResolver.LookupMX(ctx, domain)
	if err == nil && len(mx) > 0 {
		st.HasMX = true
		st.Status = "ok"
		return st
	}

	// No MX (or MX lookup failed) — postfix falls back to A; check it too.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel2()
	ips, aerr := net.DefaultResolver.LookupIPAddr(ctx2, domain)
	if aerr == nil && len(ips) > 0 {
		st.HasA = true
		st.Status = "a-only"
		return st
	}

	isNotFound := func(e error) bool {
		var dnsErr *net.DNSError
		return errors.As(e, &dnsErr) && dnsErr.IsNotFound
	}
	switch {
	case isNotFound(err) && (aerr == nil || isNotFound(aerr)):
		// Truthfully dead: nothing in DNS at all.
		st.Status = "nxdomain"
	case isNotFound(err) && aerr != nil && !isNotFound(aerr):
		// MX gone, but A lookup failed for another reason — inconclusive.
		st.Status = "tempfail"
		st.Error = aerr.Error()
	default:
		st.Status = "tempfail"
		if err != nil {
			st.Error = err.Error()
		} else if aerr != nil {
			st.Error = aerr.Error()
		}
	}
	return st
}
