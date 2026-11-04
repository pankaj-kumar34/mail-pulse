package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
// Log Service — Wails-exported API
// ──────────────────────────────────────────────

type LogService struct {
	store        *logStore
	mu           sync.Mutex
	selectedFile string
	ingested     bool
	logsDir      string
	lineIdx      lineIndex
}

func findLogsDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mail-pulse", "logs")
}

func NewLogService() *LogService {
	return &LogService{
		store:        newLogStore(),
		selectedFile: "mail.0.log",
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

func (l *LogService) IngestFile() map[string]interface{} {
	l.mu.Lock()
	if l.ingested {
		sel := l.selectedFile
		l.mu.Unlock()
		return map[string]interface{}{"ingested": false, "status": "already_ingested", "selectedFile": sel}
	}
	filePath := filepath.Join(l.logsDir, l.selectedFile)
	l.mu.Unlock()

	f, err := os.Open(filePath)
	if err != nil {
		return map[string]interface{}{"ingested": false, "status": "file_not_found"}
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if e := parsePostfixLog(line); e != nil {
			l.store.add(e)
		}
	}
	if err := scanner.Err(); err != nil {
		return map[string]interface{}{"ingested": false, "status": "scan_error", "error": err.Error()}
	}

	l.mu.Lock()
	l.ingested = true
	sel := l.selectedFile
	l.mu.Unlock()

	return map[string]interface{}{"ingested": true, "status": "complete", "selectedFile": sel}
}

func (l *LogService) RefreshFile(filename string) map[string]interface{} {
	l.mu.Lock()
	if filename != "" {
		l.selectedFile = filename
	}
	l.ingested = false
	l.mu.Unlock()

	l.store.clear()
	return l.IngestFile()
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

func (l *LogService) GetActivity(page, limit int, search, status, sortBy, sortOrder string) PaginatedLogs {
	return l.store.getPaginated(page, limit, search, status, sortBy, sortOrder)
}

func (l *LogService) GetMessages(page, limit int, search, status, sortBy, sortOrder string) PaginatedMessages {
	return l.store.getMessages(page, limit, search, status, sortBy, sortOrder)
}

func (l *LogService) GetErrors(page, limit int, errType, sortOrder string) PaginatedLogs {
	return l.store.getErrorsPaginated(page, limit, errType, sortOrder)
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

// ── Raw Log Viewer ────────────────────────────────────────────────────────────

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

func (l *LogService) GetQueueSummaries(sinceHours int, filterFrom string) []QueueSummary {
	since := int64(0)
	if sinceHours > 0 {
		since = l.store.maxTimestamp() - int64(sinceHours)*3600000
	}
	return l.store.getQueueSummaries(since, filterFrom)
}
