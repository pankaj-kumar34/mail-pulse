package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var (
	// AppVersion is the current application version. Override at build time via:
	// -ldflags "-X main.AppVersion=1.2.0"
	AppVersion = "1.0.0"

	// GitHubRepo is the owner/repo slug checked for releases. Override at build time via:
	// -ldflags "-X main.GitHubRepo=pankaj-kumar34/mail-pulse"
	GitHubRepo = "pankaj-kumar34/mail-pulse"
)

// AppVersionInfo describes the current running application version and platform metadata.
type AppVersionInfo struct {
	Version string `json:"version"`
	Repo    string `json:"repo"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// UpdateCheckResult holds the result of checking GitHub Releases for a newer version.
type UpdateCheckResult struct {
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	HasUpdate      bool   `json:"hasUpdate"`
	ReleaseName    string `json:"releaseName"`
	ReleaseNotes   string `json:"releaseNotes"`
	ReleaseUrl     string `json:"releaseUrl"`
	PublishedAt    string `json:"publishedAt"`
	AssetName      string `json:"assetName"`
	AssetUrl       string `json:"assetUrl"`
	AssetSize      int64  `json:"assetSize"`
	ChecksumUrl    string `json:"checksumUrl"`
	CanSelfUpdate  bool   `json:"canSelfUpdate"`
}

// UpdateProgress is emitted via Wails events during download, verification, and installation.
type UpdateProgress struct {
	Stage           string  `json:"stage"` // "downloading", "verifying", "extracting", "installing", "ready", "error"
	Percent         float64 `json:"percent"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Message         string  `json:"message"`
}

type githubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	PublishedAt string        `json:"published_at"`
	Draft       bool          `json:"draft"`
	Prerelease  bool          `json:"prerelease"`
	Assets      []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size"`
	BrowserDownloadURL string `json:"browser_download_url"`
	ContentType        string `json:"content_type"`
}

// UpdaterService handles checking GitHub Releases, downloading updates, verifying checksums, and self-updating the binary.
type UpdaterService struct {
	HTTPClient      *http.Client
	APIBaseURL      string
	Repo            string
	Version         string
	TargetOS        string
	TargetArch      string
	ExecutableFn    func() (string, error)
	ProgressEmit    func(progress UpdateProgress)
	UpdatedExecPath string
	FallbackAppDirs []string
}

// NewUpdaterService creates a configured UpdaterService instance.
func NewUpdaterService() *UpdaterService {
	repo := strings.TrimSpace(os.Getenv("MAILPULSE_GITHUB_REPO"))
	if repo == "" {
		repo = GitHubRepo
	}
	repo = normalizeRepoSlug(repo)

	ver := strings.TrimSpace(os.Getenv("MAILPULSE_VERSION"))
	if ver == "" {
		ver = AppVersion
	}

	apiBase := strings.TrimSpace(os.Getenv("MAILPULSE_GITHUB_API_URL"))
	if apiBase == "" {
		apiBase = "https://api.github.com"
	}

	// Clean up any leftover .old / .bak / .new binary from a previous update on startup
	go cleanupOldBinary()

	return &UpdaterService{
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		APIBaseURL:   strings.TrimRight(apiBase, "/"),
		Repo:         repo,
		Version:      NormalizeVersion(ver),
		TargetOS:     runtime.GOOS,
		TargetArch:   runtime.GOARCH,
		ExecutableFn: os.Executable,
	}
}

func normalizeRepoSlug(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimPrefix(s, "https://github.com/")
	s = strings.TrimPrefix(s, "http://github.com/")
	s = strings.TrimPrefix(s, "git@github.com:")
	s = strings.Trim(s, "/")
	if s == "" {
		return "pankaj-kumar34/mail-pulse"
	}
	return s
}

func cleanupOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	_ = os.Remove(exe + ".old")
	_ = os.Remove(exe + ".bak")
	_ = os.Remove(exe + ".new")
}

func (u *UpdaterService) emitProgress(stage string, percent float64, downloaded, total int64, msg string) {
	p := UpdateProgress{
		Stage:           stage,
		Percent:         percent,
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		Message:         msg,
	}
	if u.ProgressEmit != nil {
		u.ProgressEmit(p)
		return
	}
	if app := application.Get(); app != nil {
		app.Event.Emit("updater:progress", p)
	}
}

// GetVersionInfo returns the current running version, target repository, and OS/Arch metadata.
func (u *UpdaterService) GetVersionInfo() AppVersionInfo {
	return AppVersionInfo{
		Version: u.Version,
		Repo:    u.Repo,
		OS:      u.TargetOS,
		Arch:    u.TargetArch,
	}
}

// CheckForUpdate queries GitHub Releases for the latest release and compares it against the running version.
func (u *UpdaterService) CheckForUpdate() (*UpdateCheckResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", u.APIBaseURL, u.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update check request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", fmt.Sprintf("MailPulse-Updater/%s (%s/%s)", u.Version, u.TargetOS, u.TargetArch))

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach GitHub Releases: %w", err)
	}
	defer resp.Body.Close()

	releasePageFallback := fmt.Sprintf("https://github.com/%s/releases", u.Repo)

	// If the repo has no releases published yet (404), return cleanly with HasUpdate: false.
	if resp.StatusCode == http.StatusNotFound {
		return &UpdateCheckResult{
			CurrentVersion: u.Version,
			LatestVersion:  u.Version,
			HasUpdate:      false,
			ReleaseName:    fmt.Sprintf("v%s", u.Version),
			ReleaseNotes:   "No published releases found on GitHub yet.",
			ReleaseUrl:     releasePageFallback,
		}, nil
	}

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("GitHub API returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var rel githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to decode GitHub release response: %w", err)
	}

	latestVer := NormalizeVersion(rel.TagName)
	if latestVer == "" {
		latestVer = u.Version
	}

	hasUpdate := CompareSemver(u.Version, latestVer) < 0
	asset, canSelfUpdate := matchReleaseAsset(rel.Assets, u.TargetOS, u.TargetArch)
	checksumURL := findChecksumAssetURL(rel.Assets)

	releaseURL := rel.HTMLURL
	if releaseURL == "" {
		releaseURL = releasePageFallback
	}

	releaseName := strings.TrimSpace(rel.Name)
	if releaseName == "" {
		releaseName = rel.TagName
	}

	res := &UpdateCheckResult{
		CurrentVersion: u.Version,
		LatestVersion:  latestVer,
		HasUpdate:      hasUpdate,
		ReleaseName:    releaseName,
		ReleaseNotes:   rel.Body,
		ReleaseUrl:     releaseURL,
		PublishedAt:    rel.PublishedAt,
		ChecksumUrl:    checksumURL,
		CanSelfUpdate:  canSelfUpdate,
	}

	if asset != nil {
		res.AssetName = asset.Name
		res.AssetUrl = asset.BrowserDownloadURL
		res.AssetSize = asset.Size
	}

	return res, nil
}

// DownloadAndInstallUpdate downloads the matched release asset, optionally verifies its SHA256 checksum,
// extracts the archive if needed, and atomically replaces the running executable.
func (u *UpdaterService) DownloadAndInstallUpdate(assetURL string, assetName string, checksumURL string) error {
	u.UpdatedExecPath = ""
	assetURL = strings.TrimSpace(assetURL)
	if assetURL == "" {
		return fmt.Errorf("no downloadable update asset URL provided")
	}
	parsedURL, err := url.Parse(assetURL)
	if err != nil || (parsedURL.Scheme != "https" && parsedURL.Scheme != "http") {
		return fmt.Errorf("invalid asset download URL")
	}

	if assetName == "" {
		assetName = filepath.Base(parsedURL.Path)
	}

	lowerName := strings.ToLower(assetName)
	if strings.HasSuffix(lowerName, ".dmg") || strings.HasSuffix(lowerName, ".msi") || strings.HasSuffix(lowerName, ".deb") || strings.HasSuffix(lowerName, ".rpm") {
		return fmt.Errorf("installer package %s requires manual installation; please use Open Release Page", assetName)
	}

	tmpDir, err := os.MkdirTemp("", "mail-pulse-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary update directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, filepath.Base(assetName))
	u.emitProgress("downloading", 0, 0, 0, fmt.Sprintf("Downloading %s...", assetName))

	if err := u.downloadFileWithProgress(assetURL, archivePath); err != nil {
		u.emitProgress("error", 0, 0, 0, err.Error())
		return err
	}

	// Verify SHA256 checksum if checksums.txt URL is available
	if strings.TrimSpace(checksumURL) != "" {
		u.emitProgress("verifying", 82, 0, 0, "Verifying SHA-256 checksum...")
		if err := u.verifyAssetChecksum(archivePath, assetName, checksumURL); err != nil {
			u.emitProgress("error", 82, 0, 0, err.Error())
			return err
		}
	}

	u.emitProgress("extracting", 88, 0, 0, "Extracting update package...")
	newBinaryPath, err := extractBinaryFromAsset(archivePath, assetName, tmpDir, u.TargetOS)
	if err != nil {
		u.emitProgress("error", 88, 0, 0, err.Error())
		return err
	}

	u.emitProgress("installing", 95, 0, 0, "Installing update...")
	if err := u.replaceExecutable(newBinaryPath); err != nil {
		u.emitProgress("error", 95, 0, 0, err.Error())
		return err
	}

	u.emitProgress("ready", 100, 0, 0, "Update installed! Restart Mail Pulse to apply.")
	return nil
}

func (u *UpdaterService) downloadFileWithProgress(downloadURL string, destPath string) error {
	var transport http.RoundTripper
	if u.HTTPClient != nil {
		transport = u.HTTPClient.Transport
	}
	client := &http.Client{
		Timeout:   10 * time.Minute,
		Transport: transport,
	}
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("MailPulse-Updater/%s", u.Version))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download update asset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("asset download returned HTTP %d", resp.StatusCode)
	}

	out, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer out.Close()

	totalBytes := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)
	lastEmit := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed writing update payload: %w", writeErr)
			}
			downloaded += int64(n)
			if time.Since(lastEmit) >= 80*time.Millisecond || readErr == io.EOF {
				var pct float64
				if totalBytes > 0 {
					pct = (float64(downloaded) / float64(totalBytes)) * 80.0
				} else {
					pct = 40.0
				}
				u.emitProgress("downloading", pct, downloaded, totalBytes, "Downloading update...")
				lastEmit = time.Now()
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("error reading download stream: %w", readErr)
		}
	}

	return out.Sync()
}

func (u *UpdaterService) verifyAssetChecksum(filePath string, assetName string, checksumURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checksumURL, nil)
	if err != nil {
		return fmt.Errorf("invalid checksum URL: %w", err)
	}
	req.Header.Set("User-Agent", fmt.Sprintf("MailPulse-Updater/%s", u.Version))

	resp, err := u.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch checksums file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("checksums download returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return fmt.Errorf("failed to read checksums file: %w", err)
	}

	expectedHash := parseChecksumForAsset(string(body), assetName)
	if expectedHash == "" {
		// Asset not listed in checksums file; do not fail if checksums file doesn't list this specific file
		return nil
	}

	actualHash, err := computeFileSHA256(filePath)
	if err != nil {
		return fmt.Errorf("failed to compute SHA-256 of downloaded file: %w", err)
	}

	if !strings.EqualFold(expectedHash, actualHash) {
		return fmt.Errorf("SHA-256 checksum mismatch for %s (expected %s, got %s)", assetName, expectedHash, actualHash)
	}

	return nil
}

func parseChecksumForAsset(checksumContent string, assetName string) string {
	targetBase := filepath.Base(assetName)
	lines := strings.Split(strings.ReplaceAll(checksumContent, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hashCandidate := strings.TrimSpace(fields[0])
			nameCandidate := strings.TrimPrefix(strings.TrimSpace(fields[1]), "*")
			if filepath.Base(nameCandidate) == targetBase && len(hashCandidate) == 64 {
				return strings.ToLower(hashCandidate)
			}
		}
	}
	return ""
}

func computeFileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func extractBinaryFromAsset(archivePath string, assetName string, tmpDir string, targetOS string) (string, error) {
	lower := strings.ToLower(assetName)
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return "", err
	}

	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		if err := untarGz(archivePath, extractDir); err != nil {
			return "", fmt.Errorf("failed to extract tar.gz archive: %w", err)
		}
		return findExtractedExecutable(extractDir, targetOS)
	}

	if strings.HasSuffix(lower, ".zip") {
		if err := unzipArchive(archivePath, extractDir); err != nil {
			return "", fmt.Errorf("failed to extract zip archive: %w", err)
		}
		return findExtractedExecutable(extractDir, targetOS)
	}

	// Raw binary asset
	if err := os.Chmod(archivePath, 0755); err != nil {
		return "", fmt.Errorf("failed to set executable permissions: %w", err)
	}
	return archivePath, nil
}

func untarGz(srcPath string, destDir string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) && filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("invalid path in tar archive: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0777|0600)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(outFile, io.LimitReader(tr, 500*1024*1024))
			closeErr := outFile.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	return nil
}

func unzipArchive(srcPath string, destDir string) error {
	r, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer r.Close()

	cleanDest := filepath.Clean(destDir) + string(os.PathSeparator)

	for _, zf := range r.File {
		target := filepath.Join(destDir, zf.Name)
		if !strings.HasPrefix(filepath.Clean(target)+string(os.PathSeparator), cleanDest) && filepath.Clean(target) != filepath.Clean(destDir) {
			return fmt.Errorf("invalid path in zip archive: %s", zf.Name)
		}

		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		rc, err := zf.Open()
		if err != nil {
			return err
		}

		mode := zf.Mode() & 0777
		if mode == 0 {
			mode = 0755
		}
		outFile, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode|0600)
		if err != nil {
			rc.Close()
			return err
		}

		_, copyErr := io.Copy(outFile, io.LimitReader(rc, 500*1024*1024))
		rc.Close()
		closeErr := outFile.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func findExtractedExecutable(extractDir string, targetOS string) (string, error) {
	var bestCandidate string
	var largestFile string
	var largestSize int64

	err := filepath.Walk(extractDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info == nil || info.IsDir() {
			return nil
		}
		base := strings.ToLower(filepath.Base(path))
		normalizedPath := filepath.ToSlash(strings.ToLower(path))

		// On macOS, prioritize Contents/MacOS/mail-pulse inside an .app bundle
		if targetOS == "darwin" && strings.Contains(normalizedPath, ".app/contents/macos/") {
			bestCandidate = path
			return nil
		}

		if targetOS == "windows" && strings.HasSuffix(base, ".exe") && strings.Contains(base, "mail-pulse") {
			bestCandidate = path
			return nil
		}

		if base == "mail-pulse" || base == "mail-pulse.exe" || strings.HasPrefix(base, "mail-pulse-") {
			if !strings.HasSuffix(base, ".plist") && !strings.HasSuffix(base, ".icns") && !strings.HasSuffix(base, ".png") {
				bestCandidate = path
			}
		}

		if info.Size() > largestSize && !strings.HasSuffix(base, ".icns") && !strings.HasSuffix(base, ".car") && !strings.HasSuffix(base, ".png") {
			largestSize = info.Size()
			largestFile = path
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	chosen := bestCandidate
	if chosen == "" {
		chosen = largestFile
	}
	if chosen == "" {
		return "", fmt.Errorf("could not locate mail-pulse executable inside archive")
	}

	_ = os.Chmod(chosen, 0755)
	return chosen, nil
}

func (u *UpdaterService) replaceExecutable(newBinaryPath string) error {
	execFn := u.ExecutableFn
	if execFn == nil {
		execFn = os.Executable
	}
	currentExec, err := execFn()
	if err != nil {
		return fmt.Errorf("failed to locate running executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(currentExec); err == nil {
		currentExec = resolved
	}
	if u.TargetOS == "darwin" {
		currentExec = resolveMacAppTranslocation(currentExec)
	}

	err = u.replaceExecutableAt(newBinaryPath, currentExec)
	if err == nil {
		u.UpdatedExecPath = currentExec
		return nil
	}

	if u.TargetOS == "darwin" {
		if fallbackErr := u.installMacAppBundleFallback(newBinaryPath, currentExec); fallbackErr == nil {
			return nil
		} else {
			return fmt.Errorf("%w (fallback install to Applications also failed: %v)", err, fallbackErr)
		}
	}

	return err
}

func (u *UpdaterService) replaceExecutableAt(newBinaryPath string, targetExec string) error {
	stagedNew := targetExec + ".new"
	backupOld := targetExec + ".old"

	// Copy new binary to the same filesystem as the target executable for atomic rename
	if err := copyFileContents(newBinaryPath, stagedNew, 0755); err != nil {
		return fmt.Errorf("failed to stage updated binary (check write permissions): %w", err)
	}

	_ = os.Remove(backupOld)
	if err := os.Rename(targetExec, backupOld); err != nil {
		_ = os.Remove(stagedNew)
		return fmt.Errorf("failed to back up current binary: %w", err)
	}

	if err := os.Rename(stagedNew, targetExec); err != nil {
		// Rollback on failure
		_ = os.Rename(backupOld, targetExec)
		_ = os.Remove(stagedNew)
		return fmt.Errorf("failed to move updated binary into place: %w", err)
	}

	if u.TargetOS != "windows" {
		_ = os.Remove(backupOld)
	}

	// On macOS, if running inside a .app bundle, clear quarantine attributes and re-sign adhoc
	if u.TargetOS == "darwin" {
		if appBundle := findMacAppBundlePath(targetExec); appBundle != "" {
			_ = exec.Command("xattr", "-cr", appBundle).Run()
			_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", appBundle).Run()
		}
	}

	return nil
}

func resolveMacAppTranslocation(execPath string) string {
	if !strings.Contains(execPath, "/AppTranslocation/") {
		return execPath
	}
	out, err := exec.Command("/sbin/mount").Output()
	if err != nil {
		out, err = exec.Command("mount").Output()
		if err != nil {
			return execPath
		}
	}
	if resolved := parseTranslocatedOriginFromMount(string(out), execPath); resolved != "" {
		return resolved
	}
	return execPath
}

// parseTranslocatedOriginFromMount parses macOS `mount` output to map a Gatekeeper
// AppTranslocation executable path back to its original .app bundle path on disk.
func parseTranslocatedOriginFromMount(mountOutput string, execPath string) string {
	cleanExec := filepath.Clean(execPath)
	translocatedBundle := findMacAppBundlePath(cleanExec)
	if translocatedBundle == "" {
		return ""
	}
	relInsideBundle := strings.TrimPrefix(cleanExec, translocatedBundle)
	if relInsideBundle == "" || !strings.HasPrefix(relInsideBundle, "/") {
		return ""
	}

	lines := strings.Split(strings.ReplaceAll(mountOutput, "\r\n", "\n"), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "/AppTranslocation/") {
			continue
		}
		parenIdx := strings.LastIndex(line, " (")
		if parenIdx == -1 || !strings.HasSuffix(line, ")") {
			continue
		}
		head := line[:parenIdx]
		sepIdx := strings.LastIndex(head, " on ")
		if sepIdx == -1 {
			continue
		}

		sourcePath := strings.TrimSpace(head[:sepIdx])
		mountPoint := strings.TrimRight(filepath.Clean(strings.TrimSpace(head[sepIdx+4:])), "/")

		if !strings.HasSuffix(strings.ToLower(sourcePath), ".app") {
			continue
		}
		if strings.HasPrefix(cleanExec, mountPoint+"/") ||
			(strings.HasPrefix(cleanExec, "/var/") && strings.HasPrefix(mountPoint, "/private/var/") &&
				strings.HasPrefix("/private"+cleanExec, mountPoint+"/")) {
			return filepath.Join(sourcePath, relInsideBundle)
		}
	}

	return ""
}

func (u *UpdaterService) installMacAppBundleFallback(newBinaryPath string, currentExec string) error {
	sourceBundle := findMacAppBundlePath(newBinaryPath)
	currentBundle := findMacAppBundlePath(currentExec)

	var bundleName string
	var relExec string
	if sourceBundle != "" {
		bundleName = filepath.Base(sourceBundle)
		relExec = strings.TrimPrefix(filepath.Clean(newBinaryPath), filepath.Clean(sourceBundle))
	} else if currentBundle != "" {
		bundleName = filepath.Base(currentBundle)
		relExec = strings.TrimPrefix(filepath.Clean(currentExec), filepath.Clean(currentBundle))
	} else {
		return fmt.Errorf("no macOS .app bundle found for fallback installation")
	}

	if relExec == "" || !strings.HasPrefix(relExec, "/") {
		relExec = filepath.Join("/Contents/MacOS", filepath.Base(newBinaryPath))
	}

	dirs := u.FallbackAppDirs
	if len(dirs) == 0 {
		dirs = []string{"/Applications"}
		if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
			dirs = append(dirs, filepath.Join(home, "Applications"))
		}
	}

	var lastErr error
	for _, targetDir := range dirs {
		if strings.TrimSpace(targetDir) == "" {
			continue
		}
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			lastErr = err
			continue
		}

		destApp := filepath.Join(targetDir, bundleName)
		stagingApp := filepath.Join(targetDir, "."+bundleName+".staging")
		backupApp := filepath.Join(targetDir, "."+bundleName+".old")

		_ = os.RemoveAll(stagingApp)
		if sourceBundle != "" {
			if err := copyDirContents(sourceBundle, stagingApp); err != nil {
				_ = os.RemoveAll(stagingApp)
				lastErr = err
				continue
			}
		} else {
			if err := copyDirContents(currentBundle, stagingApp); err != nil {
				_ = os.RemoveAll(stagingApp)
				lastErr = err
				continue
			}
			if err := copyFileContents(newBinaryPath, filepath.Join(stagingApp, relExec), 0755); err != nil {
				_ = os.RemoveAll(stagingApp)
				lastErr = err
				continue
			}
		}

		destExec := filepath.Join(stagingApp, relExec)
		_ = os.Chmod(destExec, 0755)

		_ = os.RemoveAll(backupApp)
		_, destExistsErr := os.Stat(destApp)
		hadExisting := destExistsErr == nil
		if hadExisting {
			if err := os.Rename(destApp, backupApp); err != nil {
				_ = os.RemoveAll(stagingApp)
				lastErr = err
				continue
			}
		}

		if err := os.Rename(stagingApp, destApp); err != nil {
			if hadExisting {
				_ = os.Rename(backupApp, destApp)
			}
			_ = os.RemoveAll(stagingApp)
			lastErr = err
			continue
		}

		if hadExisting {
			_ = os.RemoveAll(backupApp)
		}

		_ = exec.Command("xattr", "-cr", destApp).Run()
		_ = exec.Command("codesign", "--force", "--deep", "--sign", "-", destApp).Run()

		u.UpdatedExecPath = filepath.Join(destApp, relExec)
		return nil
	}

	if lastErr == nil {
		return fmt.Errorf("no writable Applications directory available")
	}
	return lastErr
}

func copyDirContents(srcDir, dstDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dstDir, rel)

		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return err
			}
			_ = os.Remove(targetPath)
			return os.Symlink(linkTarget, targetPath)
		}

		if info.IsDir() {
			mode := info.Mode().Perm()
			if mode == 0 {
				mode = 0755
			}
			return os.MkdirAll(targetPath, mode|0700)
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0644
		}
		return copyFileContents(path, targetPath, mode)
	})
}

func findMacAppBundlePath(execPath string) string {
	clean := filepath.Clean(execPath)
	parts := strings.Split(filepath.ToSlash(clean), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if strings.HasSuffix(strings.ToLower(parts[i]), ".app") {
			prefix := strings.Join(parts[:i+1], "/")
			if strings.HasPrefix(clean, "/") && !strings.HasPrefix(prefix, "/") {
				return "/" + prefix
			}
			return prefix
		}
	}
	return ""
}

func copyFileContents(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	if err := out.Chmod(mode); err != nil {
		return err
	}
	return out.Sync()
}

// RestartApp launches a fresh instance of the updated application and exits the current process.
func (u *UpdaterService) RestartApp() error {
	exe := strings.TrimSpace(u.UpdatedExecPath)
	if exe == "" {
		execFn := u.ExecutableFn
		if execFn == nil {
			execFn = os.Executable
		}
		var err error
		exe, err = execFn()
		if err != nil {
			return fmt.Errorf("failed to resolve executable path: %w", err)
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		if u.TargetOS == "darwin" {
			exe = resolveMacAppTranslocation(exe)
		}
	}

	var cmd *exec.Cmd
	if u.TargetOS == "darwin" {
		if appBundle := findMacAppBundlePath(exe); appBundle != "" {
			_ = exec.Command("xattr", "-cr", appBundle).Run()
			cmd = exec.Command("open", "-n", appBundle)
		}
	}
	if cmd == nil {
		cmd = exec.Command(exe, os.Args[1:]...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to relaunch application: %w", err)
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		if app := application.Get(); app != nil {
			app.Quit()
		}
		os.Exit(0)
	}()

	return nil
}

// OpenReleasePage opens the given GitHub release URL in the user's default system browser.
func (u *UpdaterService) OpenReleasePage(targetURL string) error {
	targetURL = strings.TrimSpace(targetURL)
	if targetURL == "" {
		targetURL = fmt.Sprintf("https://github.com/%s/releases", u.Repo)
	}
	parsed, err := url.Parse(targetURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("invalid release URL")
	}

	var cmd *exec.Cmd
	switch u.TargetOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		cmd = exec.Command("xdg-open", targetURL)
	}
	return cmd.Start()
}

// NormalizeVersion strips a leading "v" or "V" and whitespace.
func NormalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return strings.TrimSpace(v)
}

// CompareSemver compares two semantic version strings (e.g. "1.0.0" vs "1.1.0").
// Returns -1 if v1 < v2, 0 if v1 == v2, and +1 if v1 > v2.
func CompareSemver(v1, v2 string) int {
	n1 := NormalizeVersion(v1)
	n2 := NormalizeVersion(v2)
	if n1 == n2 {
		return 0
	}

	core1, pre1 := splitPrerelease(n1)
	core2, pre2 := splitPrerelease(n2)

	parts1 := parseNumericParts(core1)
	parts2 := parseNumericParts(core2)

	for i := 0; i < 3; i++ {
		if parts1[i] < parts2[i] {
			return -1
		}
		if parts1[i] > parts2[i] {
			return 1
		}
	}

	// Stable release > pre-release (e.g. 1.0.0 > 1.0.0-beta.1)
	if pre1 == "" && pre2 != "" {
		return 1
	}
	if pre1 != "" && pre2 == "" {
		return -1
	}
	return strings.Compare(pre1, pre2)
}

func splitPrerelease(v string) (string, string) {
	if idx := strings.IndexByte(v, '+'); idx != -1 {
		v = v[:idx]
	}
	if idx := strings.IndexByte(v, '-'); idx != -1 {
		return v[:idx], v[idx+1:]
	}
	return v, ""
}

func parseNumericParts(core string) [3]int {
	var out [3]int
	segs := strings.Split(core, ".")
	for i := 0; i < len(segs) && i < 3; i++ {
		n, err := strconv.Atoi(segs[i])
		if err == nil {
			out[i] = n
		}
	}
	return out
}

func findChecksumAssetURL(assets []githubAsset) string {
	for _, a := range assets {
		low := strings.ToLower(a.Name)
		if low == "checksums.txt" || low == "sha256sums" || low == "sha256sums.txt" || strings.HasSuffix(low, "_checksums.txt") {
			return a.BrowserDownloadURL
		}
	}
	return ""
}

func matchReleaseAsset(assets []githubAsset, goos, goarch string) (*githubAsset, bool) {
	type scoredAsset struct {
		asset         githubAsset
		score         int
		canSelfUpdate bool
	}

	var candidates []scoredAsset

	for _, a := range assets {
		low := strings.ToLower(a.Name)
		if strings.HasSuffix(low, ".txt") || strings.HasSuffix(low, ".sha256") || strings.HasSuffix(low, ".sig") || strings.HasSuffix(low, ".asc") {
			continue
		}

		if !matchesOS(low, goos) {
			continue
		}
		archScore, archMatched := matchesArch(low, goos, goarch)
		if !archMatched {
			continue
		}

		score := archScore
		canSelf := true

		switch {
		case strings.HasSuffix(low, ".tar.gz") || strings.HasSuffix(low, ".tgz"):
			score += 30
		case strings.HasSuffix(low, ".zip"):
			score += 28
		case strings.HasSuffix(low, ".exe"):
			if strings.Contains(low, "setup") || strings.Contains(low, "installer") {
				score += 10
				canSelf = false
			} else {
				score += 25
			}
		case strings.HasSuffix(low, ".dmg") || strings.HasSuffix(low, ".msi") || strings.HasSuffix(low, ".deb") || strings.HasSuffix(low, ".rpm"):
			score += 5
			canSelf = false
		default:
			score += 20
		}

		candidates = append(candidates, scoredAsset{
			asset:         a,
			score:         score,
			canSelfUpdate: canSelf,
		})
	}

	if len(candidates) == 0 {
		return nil, false
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return &best.asset, best.canSelfUpdate
}

func matchesOS(name string, goos string) bool {
	switch goos {
	case "darwin":
		return strings.Contains(name, "darwin") || strings.Contains(name, "macos") || strings.Contains(name, "osx") || strings.HasSuffix(name, ".dmg")
	case "windows":
		return strings.Contains(name, "windows") || strings.Contains(name, "win64") || strings.Contains(name, "win32") || strings.HasSuffix(name, ".exe") || strings.HasSuffix(name, ".msi")
	case "linux":
		return strings.Contains(name, "linux") || strings.HasSuffix(name, ".appimage") || strings.HasSuffix(name, ".deb") || strings.HasSuffix(name, ".rpm")
	default:
		return strings.Contains(name, goos)
	}
}

func matchesArch(name string, goos string, goarch string) (int, bool) {
	switch goarch {
	case "arm64":
		if strings.Contains(name, "arm64") || strings.Contains(name, "aarch64") {
			return 20, true
		}
		if goos == "darwin" && (strings.Contains(name, "universal") || strings.Contains(name, "all")) {
			return 15, true
		}
		if !containsAnyArchToken(name) {
			return 5, true
		}
		return 0, false
	case "amd64":
		if strings.Contains(name, "amd64") || strings.Contains(name, "x86_64") || strings.Contains(name, "x64") {
			return 20, true
		}
		if goos == "darwin" && (strings.Contains(name, "universal") || strings.Contains(name, "all")) {
			return 15, true
		}
		if !containsAnyArchToken(name) {
			return 5, true
		}
		return 0, false
	default:
		if strings.Contains(name, goarch) {
			return 20, true
		}
		return 0, false
	}
}

func containsAnyArchToken(name string) bool {
	tokens := []string{"arm64", "aarch64", "amd64", "x86_64", "x64", "386", "i386"}
	for _, t := range tokens {
		if strings.Contains(name, t) {
			return true
		}
	}
	return false
}
