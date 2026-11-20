package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Regression: switching files while an ingest is running must not leave the old
// file's lines in the store (fixed by abort-before-clear + per-line gen check).
func TestRefreshFileNoPollution(t *testing.T) {
	dir := t.TempDir()
	var a, b strings.Builder
	for i := 0; i < 100000; i++ {
		fmt.Fprintf(&a, "Nov 19 10:00:%02d host postfix/smtp[100]: AA%03dAA11: to=<a%d@x.com>, relay=m[1.2.3.4]:25, delay=1, delays=0/0/0/1, dsn=4.0.0, status=deferred (connect fail %d)\n", i%60, i%900, i, i)
	}
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&b, "Nov 20 11:00:%02d host postfix/smtp[200]: BB%03dBB22: to=<b%d@y.com>, relay=n[5.6.7.8]:25, delay=1, delays=0/0/0/1, dsn=2.0.0, status=sent (250 ok %d)\n", i%60, i, i, i)
	}
	if err := os.WriteFile(filepath.Join(dir, "A.log"), []byte(a.String()), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "B.log"), []byte(b.String()), 0644); err != nil {
		t.Fatal(err)
	}

	svc := NewLogService()
	svc.logsDir = dir
	svc.selectedFile = "A.log"

	svc.StartIngest()
	time.Sleep(30 * time.Millisecond) // A's ingest is now mid-file
	svc.RefreshFile("B.log")

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if s := svc.istate.snapshot(); s["done"] == true && s["running"] == false {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond) // let a superseded goroutine fully exit

	svc.store.mu.RLock()
	total := len(svc.store.logs)
	polluted := 0
	for i := range svc.store.logs {
		if strings.Contains(svc.store.logs[i].ID, "AA") {
			polluted++
		}
	}
	svc.store.mu.RUnlock()

	if polluted != 0 {
		t.Errorf("store polluted with %d entries from file A after switching to B", polluted)
	}
	if total != 200 {
		t.Errorf("store has %d entries, want exactly 200 (file B)", total)
	}
}

// Regression: rotating/truncating the file must rebuild the line index, and the
// index must not count a phantom trailing line.
func TestLineIndexShrinkRebuilds(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "live.log")
	os.WriteFile(p, []byte("l1\nl2\nl3\nl4\n"), 0644)

	svc := &LogService{logsDir: dir}
	svc.lineIdx.filename = "live.log"
	if err := svc.lineIdx.build(p); err != nil {
		t.Fatal(err)
	}
	if got := svc.lineIdx.totalLines(); got != 4 {
		t.Fatalf("build: totalLines = %d, want 4 (no phantom trailing line)", got)
	}

	// Rotation: file replaced by a shorter one.
	os.WriteFile(p, []byte("new1\nnew2\n"), 0644)
	added, reset, err := svc.lineIdx.extend(p)
	if err != nil {
		t.Fatal(err)
	}
	if !reset {
		t.Fatalf("shrink did not signal reset")
	}
	if got := svc.lineIdx.totalLines(); got != 2 {
		t.Fatalf("after rotation: totalLines = %d, want 2", got)
	}

	// Live append after rotation still works.
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("new3\n")
	f.Close()
	added, reset, err = svc.lineIdx.extend(p)
	if err != nil || reset || added != 1 {
		t.Fatalf("append after rotation: added=%d reset=%v err=%v, want 1/false/nil", added, reset, err)
	}
	if got := svc.lineIdx.totalLines(); got != 3 {
		t.Fatalf("after append: totalLines = %d, want 3", got)
	}
	res := svc.GetLines("live.log", 2, 1)
	if len(res.Lines) != 1 || res.Lines[0] != "new3" {
		t.Fatalf("GetLines(2) = %v, want [new3]", res.Lines)
	}
}

// Regression: files without a trailing newline still count their last line.
func TestLineIndexNoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.log")
	os.WriteFile(p, []byte("a\nb"), 0644)

	idx := &lineIndex{}
	if err := idx.build(p); err != nil {
		t.Fatal(err)
	}
	if got := idx.totalLines(); got != 2 {
		t.Fatalf("totalLines = %d, want 2", got)
	}
}
