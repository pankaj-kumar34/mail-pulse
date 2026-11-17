package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type handlerRoundTripper struct {
	handler http.HandlerFunc
}

func (h handlerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	h.handler(rec, req)
	return rec.Result(), nil
}

func newTestHTTPClient(fn http.HandlerFunc) *http.Client {
	return &http.Client{
		Transport: handlerRoundTripper{handler: fn},
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		v1, v2 string
		want   int
	}{
		{"1.0.0", "v1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "1.2.0", -1},
		{"0.9.9", "1.0.0", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0-beta.1", 1},
		{"1.0.0-alpha.1", "1.0.0-beta.1", -1},
	}
	for _, tc := range cases {
		if got := CompareSemver(tc.v1, tc.v2); got != tc.want {
			t.Errorf("CompareSemver(%q, %q) = %d, want %d", tc.v1, tc.v2, got, tc.want)
		}
	}
}

func TestNormalizeRepoSlug(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/pankaj-kumar34/mail-pulse.git", "pankaj-kumar34/mail-pulse"},
		{"pankaj-kumar34/mail-pulse", "pankaj-kumar34/mail-pulse"},
		{"git@github.com:pankaj-kumar34/mail-pulse.git", "pankaj-kumar34/mail-pulse"},
	}
	for _, tc := range cases {
		if got := normalizeRepoSlug(tc.in); got != tc.want {
			t.Errorf("normalizeRepoSlug(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCheckForUpdate_UpdateAvailable(t *testing.T) {
	updater := NewUpdaterService()
	updater.HTTPClient = newTestHTTPClient(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/pankaj-kumar34/mail-pulse/releases/latest" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		rel := githubRelease{
			TagName:     "v1.2.0",
			Name:        "Mail Pulse v1.2.0",
			Body:        "### What's New\n- Added GitHub Releases Auto-Updater",
			HTMLURL:     "https://github.com/pankaj-kumar34/mail-pulse/releases/tag/v1.2.0",
			PublishedAt: "2026-11-19T12:00:00Z",
			Assets: []githubAsset{
				{
					Name:               "mail-pulse-darwin-arm64.dmg",
					Size:               42000000,
					BrowserDownloadURL: "https://example.com/mail-pulse-darwin-arm64.dmg",
				},
				{
					Name:               "mail-pulse-darwin-arm64.tar.gz",
					Size:               18500000,
					BrowserDownloadURL: "https://example.com/mail-pulse-darwin-arm64.tar.gz",
				},
				{
					Name:               "checksums.txt",
					Size:               512,
					BrowserDownloadURL: "https://example.com/checksums.txt",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(rel)
	})
	updater.Repo = "pankaj-kumar34/mail-pulse"
	updater.Version = "1.0.0"
	updater.TargetOS = "darwin"
	updater.TargetArch = "arm64"

	res, err := updater.CheckForUpdate()
	if err != nil {
		t.Fatalf("CheckForUpdate error: %v", err)
	}
	if !res.HasUpdate {
		t.Error("expected HasUpdate=true")
	}
	if res.CurrentVersion != "1.0.0" || res.LatestVersion != "1.2.0" {
		t.Errorf("versions = %q -> %q, want 1.0.0 -> 1.2.0", res.CurrentVersion, res.LatestVersion)
	}
	if res.AssetName != "mail-pulse-darwin-arm64.tar.gz" {
		t.Errorf("AssetName = %q, want mail-pulse-darwin-arm64.tar.gz", res.AssetName)
	}
	if !res.CanSelfUpdate {
		t.Error("expected CanSelfUpdate=true")
	}
	if res.ChecksumUrl != "https://example.com/checksums.txt" {
		t.Errorf("ChecksumUrl = %q", res.ChecksumUrl)
	}
}

func TestCheckForUpdate_NoReleases404(t *testing.T) {
	updater := NewUpdaterService()
	updater.HTTPClient = newTestHTTPClient(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	updater.Version = "1.0.0"

	res, err := updater.CheckForUpdate()
	if err != nil {
		t.Fatalf("CheckForUpdate error: %v", err)
	}
	if res.HasUpdate {
		t.Error("expected HasUpdate=false on 404")
	}
	if res.LatestVersion != "1.0.0" {
		t.Errorf("LatestVersion = %q, want 1.0.0", res.LatestVersion)
	}
}

func TestDownloadAndInstallUpdate_EndToEnd(t *testing.T) {
	updatedPayload := []byte("#!/bin/sh\necho mail-pulse-v1.2.0\n")
	var tarGzBuf bytes.Buffer
	gzw := gzip.NewWriter(&tarGzBuf)
	tw := tar.NewWriter(gzw)

	hdr := &tar.Header{
		Name: "Mail Pulse.app/Contents/MacOS/mail-pulse",
		Mode: 0755,
		Size: int64(len(updatedPayload)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write(updatedPayload); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tw.Close: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("gzw.Close: %v", err)
	}

	archiveBytes := tarGzBuf.Bytes()
	hash := sha256.Sum256(archiveBytes)
	expectedHex := hex.EncodeToString(hash[:])

	tmpDir := t.TempDir()
	fakeExecPath := filepath.Join(tmpDir, "mail-pulse")
	if err := os.WriteFile(fakeExecPath, []byte("old-binary"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var stages []string
	updater := NewUpdaterService()
	updater.HTTPClient = newTestHTTPClient(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mail-pulse-darwin-arm64.tar.gz":
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(archiveBytes)))
			_, _ = w.Write(archiveBytes)
		case "/checksums.txt":
			_, _ = fmt.Fprintf(w, "%s  mail-pulse-darwin-arm64.tar.gz\n", expectedHex)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})
	updater.TargetOS = "darwin"
	updater.TargetArch = "arm64"
	updater.ExecutableFn = func() (string, error) {
		return fakeExecPath, nil
	}
	updater.ProgressEmit = func(p UpdateProgress) {
		stages = append(stages, p.Stage)
	}

	err := updater.DownloadAndInstallUpdate(
		"https://example.com/mail-pulse-darwin-arm64.tar.gz",
		"mail-pulse-darwin-arm64.tar.gz",
		"https://example.com/checksums.txt",
	)
	if err != nil {
		t.Fatalf("DownloadAndInstallUpdate error: %v", err)
	}

	installedBytes, err := os.ReadFile(fakeExecPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(installedBytes, updatedPayload) {
		t.Errorf("installed binary mismatch: got %q, want %q", installedBytes, updatedPayload)
	}

	for _, reqStage := range []string{"downloading", "verifying", "extracting", "installing", "ready"} {
		found := false
		for _, s := range stages {
			if s == reqStage {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing progress stage %q in %v", reqStage, stages)
		}
	}
}

func TestDownloadAndInstallUpdate_ChecksumMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	fakeExecPath := filepath.Join(tmpDir, "mail-pulse")
	if err := os.WriteFile(fakeExecPath, []byte("old-binary"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	updater := NewUpdaterService()
	updater.HTTPClient = newTestHTTPClient(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/mail-pulse-linux-amd64.tar.gz":
			_, _ = w.Write([]byte("corrupted-payload"))
		case "/checksums.txt":
			_, _ = fmt.Fprintf(w, "%s  mail-pulse-linux-amd64.tar.gz\n", strings.Repeat("a", 64))
		}
	})
	updater.TargetOS = "linux"
	updater.TargetArch = "amd64"
	updater.ExecutableFn = func() (string, error) {
		return fakeExecPath, nil
	}

	err := updater.DownloadAndInstallUpdate(
		"https://example.com/mail-pulse-linux-amd64.tar.gz",
		"mail-pulse-linux-amd64.tar.gz",
		"https://example.com/checksums.txt",
	)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got: %v", err)
	}

	afterBytes, _ := os.ReadFile(fakeExecPath)
	if string(afterBytes) != "old-binary" {
		t.Errorf("binary was modified despite checksum failure: %q", afterBytes)
	}
}

func TestParseTranslocatedOriginFromMount(t *testing.T) {
	mountOut := `/dev/disk3s1s1 on / (apfs, sealed, local, read-only, journaled)
/Users/test/Downloads/Mail Pulse.app on /private/var/folders/xx/yy/T/AppTranslocation/1234-ABCD/d/Mail Pulse.app (apfs, local, nodev, nosuid, read-only)`

	transExec := "/var/folders/xx/yy/T/AppTranslocation/1234-ABCD/d/Mail Pulse.app/Contents/MacOS/mail-pulse"
	got := parseTranslocatedOriginFromMount(mountOut, transExec)
	want := "/Users/test/Downloads/Mail Pulse.app/Contents/MacOS/mail-pulse"
	if got != want {
		t.Errorf("parseTranslocatedOriginFromMount = %q, want %q", got, want)
	}
}
