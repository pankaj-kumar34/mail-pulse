package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

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
	idx.offsets = append(idx.offsets, 0) // first line starts at 0
	for {
		line, err := buf.ReadBytes('\n')
		offset += int64(len(line))
		if err != nil {
			break
		}
		idx.offsets = append(idx.offsets, offset)
	}
	return nil
}

// extend appends any new lines written since last index build.
func (idx *lineIndex) extend(path string) (int, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}

	idx.mu.RLock()
	currentSize := idx.fileSize
	idx.mu.RUnlock()

	if info.Size() <= currentSize {
		return 0, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := f.Seek(currentSize, io.SeekStart); err != nil {
		return 0, err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	added := 0
	offset := currentSize
	buf := bufio.NewReaderSize(f, 64*1024)
	for {
		line, err := buf.ReadBytes('\n')
		offset += int64(len(line))
		if len(line) > 0 && err == nil {
			idx.offsets = append(idx.offsets, offset)
			added++
		}
		if err != nil {
			break
		}
	}
	idx.fileSize = info.Size()
	return added, nil
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

func prefsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mail-pulse", "prefs.json")
}

func loadSelectedFile() string {
	data, err := os.ReadFile(prefsPath())
	if err != nil {
		return "mail.0.log"
	}
	var prefs struct {
		SelectedFile string `json:"selected_file"`
	}
	if err := json.Unmarshal(data, &prefs); err != nil || prefs.SelectedFile == "" {
		return "mail.0.log"
	}
	return prefs.SelectedFile
}

func saveSelectedFile(file string) {
	data, _ := json.Marshal(map[string]string{"selected_file": file})
	_ = os.WriteFile(prefsPath(), data, 0644)
}

func NewLogService() *LogService {
	return &LogService{
		store:        newLogStore(),
		selectedFile: loadSelectedFile(),
		logsDir:      findLogsDir(),
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
				l.store.add(e)
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
			// Pre-compute queue summaries for all time windows so GetQueueSummaries is O(1).
			for _, hours := range []int{0, 1, 6, 24} {
				since := int64(0)
				if hours > 0 {
					since = l.store.maxTimestamp() - int64(hours)*3600000
				}
				l.store.setCachedSummaries(hours, l.store.getQueueSummaries(since, ""))
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
	l.store.clear()
}

// RefreshFile resets the selected file and starts an async ingest.
// Returns immediately with {"started": true, "selectedFile": <name>}.
func (l *LogService) RefreshFile(filename string) map[string]interface{} {
	l.ResetForFile(filename)
	return l.StartIngest()
}

// ── Metrics & Insights ────────────────────────────────────────────────────────

func (l *LogService) GetMetrics(sinceHours int) Metrics {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getMetrics(since)
}

func (l *LogService) GetVolumeInsights(sinceHours int) VolumeInsights {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getVolumeInsights(since)
}

func (l *LogService) GetDelayInsights(sinceHours int) DelayInsights {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getDelayInsights(since)
}

// GetLogTimeRange returns the earliest and latest timestamps (unix ms) in the loaded log.
func (l *LogService) GetLogTimeRange() map[string]int64 {
	l.store.mu.RLock()
	defer l.store.mu.RUnlock()
	var minTs, maxTs int64
	for i := range l.store.logs {
		ts := l.store.logs[i].Timestamp
		if ts == 0 { continue }
		if minTs == 0 || ts < minTs { minTs = ts }
		if ts > maxTs               { maxTs = ts }
	}
	return map[string]int64{"min": minTs, "max": maxTs}
}

// ── Data Access ───────────────────────────────────────────────────────────────

func (l *LogService) GetActivity(sinceHours, page, limit int, search, status, sortBy, sortOrder string) PaginatedLogs {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getPaginated(since, page, limit, search, status, sortBy, sortOrder)
}

func (l *LogService) GetMessages(sinceHours, page, limit int, search, status, sortBy, sortOrder string) PaginatedMessages {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getMessages(since, page, limit, search, status, sortBy, sortOrder)
}

func (l *LogService) GetErrors(sinceHours, page, limit int, errType, sortOrder string) PaginatedLogs {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getErrorsPaginated(since, page, limit, errType, sortOrder)
}

func (l *LogService) GetRecipientDrilldown(domain string, sinceHours int) RecipientDrilldown {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getRecipientDrilldown(domain, since)
}

func (l *LogService) GetQueueAgeDistribution(sinceHours int) QueueAgeDistribution {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getQueueAgeDistribution(since)
}

func (l *LogService) GetRelayInsights(sinceHours int) RelayInsights {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getRelayInsights(since)
}

func (l *LogService) GetRelayMessages(relay, status string, sinceHours int) []RelayMessage {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getRelayMessages(relay, status, since)
}

func (l *LogService) GetSenderStats(sinceHours int) []SenderStat {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getSenderStats(since)
}

func (l *LogService) GetMessagesByReason(reason, status string, sinceHours int) []ReasonMessage {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getMessagesByReason(reason, status, since)
}

func (l *LogService) GetMessagesBySender(from, status string, sinceHours int) []ReasonMessage {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getMessagesBySender(from, status, since)
}

func (l *LogService) GetIPReputationReport(sinceHours int) IPReputationReport {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getIPReputationReport(since)
}

func (l *LogService) GetBlockEvents(ourIP, service string, sinceHours int) []BlockEvent {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getBlockEvents(ourIP, service, since)
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

	added, err := l.lineIdx.extend(path)
	if err != nil || added == 0 {
		return PollNewLinesResult{Lines: []string{}, Total: prevTotal}
	}

	result := l.GetLines(filename, prevTotal, added)
	return PollNewLinesResult{Lines: result.Lines, Total: result.Total, NewCount: added}
}

// GetRawLog returns the last 5000 lines of the file as a single string.
func (l *LogService) GetRawLog(filename string) string {
	if filename == "" {
		filename = l.selectedFile
	}
	path := filepath.Join(l.logsDir, filename)

	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	const maxLines = 5000
	const chunkSize = int64(512 * 1024)

	info, err := f.Stat()
	if err != nil {
		return ""
	}
	fileSize := info.Size()

	var buf []byte
	offset := fileSize
	lines := 0

	for offset > 0 && lines < maxLines+1 {
		read := chunkSize
		if offset < read { read = offset }
		offset -= read

		chunk := make([]byte, read)
		if _, err := f.ReadAt(chunk, offset); err != nil {
			break
		}
		buf = append(chunk, buf...)
		lines = bytes.Count(buf, []byte("\n"))
	}

	result := buf
	if lines > maxLines {
		count := 0
		idx := len(result)
		for idx > 0 {
			idx--
			if result[idx] == '\n' {
				count++
				if count == maxLines {
					result = result[idx+1:]
					break
				}
			}
		}
	}
	return string(result)
}

// GetRawLogFrom returns lines appended after byteOffset and the new file size.
func (l *LogService) GetRawLogFrom(filename string, byteOffset int64) map[string]interface{} {
	if filename == "" {
		filename = l.selectedFile
	}
	path := filepath.Join(l.logsDir, filename)

	f, err := os.Open(path)
	if err != nil {
		return map[string]interface{}{"lines": "", "offset": byteOffset, "reset": false}
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return map[string]interface{}{"lines": "", "offset": byteOffset, "reset": false}
	}
	fileSize := info.Size()

	if fileSize < byteOffset {
		return map[string]interface{}{"lines": "", "offset": int64(0), "reset": true}
	}
	if fileSize == byteOffset {
		return map[string]interface{}{"lines": "", "offset": byteOffset, "reset": false}
	}

	const maxRead = int64(512 * 1024)
	newBytes := fileSize - byteOffset
	if newBytes > maxRead { newBytes = maxRead }

	buf := make([]byte, newBytes)
	if _, err = f.ReadAt(buf, byteOffset); err != nil {
		return map[string]interface{}{"lines": "", "offset": byteOffset, "reset": false}
	}

	newOffset := byteOffset + newBytes
	return map[string]interface{}{
		"lines":  string(buf),
		"offset": newOffset,
		"reset":  false,
		"more":   newOffset < fileSize,
	}
}

func (l *LogService) GetHeatmap(sinceHours int) []HeatmapCell {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getHeatmap(since)
}

func (l *LogService) GetRetryTimeline(queueID string) RetryTimeline {
	return l.store.getRetryTimeline(queueID)
}

func (l *LogService) GetTopDeferredQueues(sinceHours, limit int) []string {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getTopDeferredQueues(since, limit)
}

func (l *LogService) GetQueueSummaries(sinceHours, page, limit int, filterFrom, statusFilter string) PaginatedQueueSummaries {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 100
	}

	// Pull full slice — from cache (O(1)) or on-demand fallback.
	var all []QueueSummary
	if filterFrom == "" {
		all = l.store.getCachedSummaries(sinceHours)
	}
	if all == nil {
		since := int64(0)
		if sinceHours > 0 {
			since = l.store.maxTimestamp() - int64(sinceHours)*3600000
		}
		all = l.store.getQueueSummaries(since, filterFrom)
	}

	// Count unknowns across the full dataset (before status filter) so the
	// frontend can show a rotation banner without needing to scan all pages.
	unknownCount := 0
	for i := range all {
		if all[i].FinalStatus == "unknown" {
			unknownCount++
		}
	}

	// Apply status filter server-side so pagination is correct per-filter.
	if statusFilter != "" {
		filtered := all[:0]
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
