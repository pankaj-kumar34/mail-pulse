package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/credentials/ibmiam"
	"github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	"github.com/IBM/ibm-cos-sdk-go/service/s3/s3manager"
)

// ── Config types ─────────────────────────────────────────────────────────────

type COSBucket struct {
	Name     string `json:"name"`
	Bucket   string `json:"bucket"`
	Endpoint string `json:"endpoint"`
}

type COSSource struct {
	Name       string      `json:"name"`
	APIKey     string      `json:"api_key"`
	InstanceID string      `json:"instance_id"`
	Buckets    []COSBucket `json:"buckets"`
}

type COSConfig struct {
	Sources []COSSource `json:"sources"`
}

// ── Download status ───────────────────────────────────────────────────────────

type DownloadFile struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // pending | downloading | done | error
	Message string `json:"message,omitempty"`
}

type DownloadStatus struct {
	Phase   string         `json:"phase"` // idle | checking | downloading | merging | loading | done | error
	Files   []DownloadFile `json:"files"`
	Message string         `json:"message,omitempty"`
}

// ── Service ───────────────────────────────────────────────────────────────────

type COSService struct {
	mu         sync.Mutex
	logSvc     *LogService
	configPath string
	dlStatus   DownloadStatus
}

func NewCOSService(logSvc *LogService) *COSService {
	home, _ := os.UserHomeDir()
	return &COSService{
		logSvc:     logSvc,
		configPath: filepath.Join(home, ".mail-pulse", "config.json"),
		dlStatus:   DownloadStatus{Phase: "idle"},
	}
}

// ── Config ────────────────────────────────────────────────────────────────────

func (c *COSService) GetCOSConfig() COSConfig {
	data, err := os.ReadFile(c.configPath)
	if err != nil {
		return COSConfig{Sources: []COSSource{}}
	}
	var cfg COSConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return COSConfig{Sources: []COSSource{}}
	}
	if cfg.Sources == nil {
		cfg.Sources = []COSSource{}
	}
	return cfg
}

func (c *COSService) SaveCOSConfig(cfg COSConfig) error {
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.configPath, data, 0600)
}

// ── S3 client ─────────────────────────────────────────────────────────────────

func (c *COSService) newClient(src COSSource, bucket COSBucket) (*s3.S3, error) {
	if src.APIKey == "" || src.InstanceID == "" {
		return nil, fmt.Errorf("COS credentials not configured for source %q", src.Name)
	}
	authEndpoint := "https://iam.cloud.ibm.com/identity/token"
	conf := aws.NewConfig().
		WithEndpoint(bucket.Endpoint).
		WithCredentials(ibmiam.NewStaticCredentials(aws.NewConfig(), authEndpoint, src.APIKey, src.InstanceID)).
		WithS3ForcePathStyle(true)
	sess, err := session.NewSession(conf)
	if err != nil {
		return nil, err
	}
	return s3.New(sess), nil
}

func (c *COSService) findSource(cfg COSConfig, sourceName string) (COSSource, bool) {
	for _, s := range cfg.Sources {
		if s.Name == sourceName {
			return s, true
		}
	}
	return COSSource{}, false
}

func (c *COSService) findBucket(src COSSource, bucketName string) (COSBucket, bool) {
	for _, b := range src.Buckets {
		if b.Name == bucketName {
			return b, true
		}
	}
	return COSBucket{}, false
}

// ── List dates ────────────────────────────────────────────────────────────────

func (c *COSService) ListCOSDates(sourceName, bucketName string) ([]string, error) {
	cfg := c.GetCOSConfig()
	src, ok := c.findSource(cfg, sourceName)
	if !ok {
		return nil, fmt.Errorf("source %q not found", sourceName)
	}
	bkt, ok := c.findBucket(src, bucketName)
	if !ok {
		return nil, fmt.Errorf("bucket %q not found in source %q", bucketName, sourceName)
	}

	client, err := c.newClient(src, bkt)
	if err != nil {
		return nil, err
	}

	seen := map[string]struct{}{}
	var dates []string
	err = client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bkt.Bucket),
	}, func(page *s3.ListObjectsV2Output, _ bool) bool {
		for _, obj := range page.Contents {
			key := aws.StringValue(obj.Key)
			base := filepath.Base(key)
			parts := strings.SplitN(base, ".", 2)
			if len(parts) < 2 {
				continue
			}
			date := parts[0]
			if _, exists := seen[date]; !exists {
				seen[date] = struct{}{}
				dates = append(dates, date)
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates, nil
}

// ── List files for a date ─────────────────────────────────────────────────────

func (c *COSService) ListCOSFiles(sourceName, bucketName, date string) ([]string, error) {
	cfg := c.GetCOSConfig()
	src, ok := c.findSource(cfg, sourceName)
	if !ok {
		return nil, fmt.Errorf("source %q not found", sourceName)
	}
	bkt, ok := c.findBucket(src, bucketName)
	if !ok {
		return nil, fmt.Errorf("bucket %q not found", bucketName)
	}

	client, err := c.newClient(src, bkt)
	if err != nil {
		return nil, err
	}

	var files []string
	err = client.ListObjectsV2Pages(&s3.ListObjectsV2Input{
		Bucket: aws.String(bkt.Bucket),
	}, func(page *s3.ListObjectsV2Output, _ bool) bool {
		for _, obj := range page.Contents {
			key := aws.StringValue(obj.Key)
			base := filepath.Base(key)
			if strings.HasPrefix(base, date+".") && strings.HasSuffix(base, ".zip") {
				files = append(files, key)
			}
		}
		return true
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// ── Download ──────────────────────────────────────────────────────────────────

func (c *COSService) GetDownloadStatus() DownloadStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.dlStatus
}

func (c *COSService) setStatus(phase string, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.dlStatus.Phase = phase
	c.dlStatus.Message = msg
}

func (c *COSService) setFileStatus(name, status, msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.dlStatus.Files {
		if c.dlStatus.Files[i].Name == name {
			c.dlStatus.Files[i].Status = status
			c.dlStatus.Files[i].Message = msg
			return
		}
	}
}

func (c *COSService) DownloadLogs(sourceName, bucketName, date string, keys []string) error {
	cfg := c.GetCOSConfig()
	src, ok := c.findSource(cfg, sourceName)
	if !ok {
		return fmt.Errorf("source %q not found", sourceName)
	}
	bkt, ok := c.findBucket(src, bucketName)
	if !ok {
		return fmt.Errorf("bucket %q not found", bucketName)
	}

	home, _ := os.UserHomeDir()
	dlDir := filepath.Join(home, ".mail-pulse", "downloads", "logs", sourceName, bucketName, date)

	// Init status
	c.mu.Lock()
	c.dlStatus = DownloadStatus{
		Phase: "checking",
		Files: make([]DownloadFile, len(keys)),
	}
	for i, k := range keys {
		c.dlStatus.Files[i] = DownloadFile{Name: filepath.Base(k), Status: "pending"}
	}
	c.mu.Unlock()

	// Check if all zips already present
	allPresent := true
	for _, k := range keys {
		zipPath := filepath.Join(dlDir, filepath.Base(k))
		if _, err := os.Stat(zipPath); err != nil {
			allPresent = false
			break
		}
	}

	if !allPresent {
		_ = os.RemoveAll(dlDir)
		if err := os.MkdirAll(dlDir, 0755); err != nil {
			c.setStatus("error", err.Error())
			return err
		}

		client, err := c.newClient(src, bkt)
		if err != nil {
			c.setStatus("error", err.Error())
			return err
		}
		downloader := s3manager.NewDownloaderWithClient(client)

		c.setStatus("downloading", "")
		for _, key := range keys {
			base := filepath.Base(key)
			c.setFileStatus(base, "downloading", "")
			zipPath := filepath.Join(dlDir, base)

			f, err := os.Create(zipPath)
			if err != nil {
				c.setFileStatus(base, "error", err.Error())
				c.setStatus("error", fmt.Sprintf("failed to create %s: %v", base, err))
				return err
			}
			_, err = downloader.Download(f, &s3.GetObjectInput{
				Bucket: aws.String(bkt.Bucket),
				Key:    aws.String(key),
			})
			f.Close()
			if err != nil {
				c.setFileStatus(base, "error", err.Error())
				c.setStatus("error", fmt.Sprintf("failed to download %s: %v", base, err))
				return err
			}
			c.setFileStatus(base, "done", "")
		}
	} else {
		c.mu.Lock()
		for i := range c.dlStatus.Files {
			c.dlStatus.Files[i].Status = "done"
			c.dlStatus.Files[i].Message = "cached"
		}
		c.mu.Unlock()
	}

	// Unzip
	c.setStatus("merging", "Unzipping…")
	for _, key := range keys {
		base := filepath.Base(key)
		zipPath := filepath.Join(dlDir, base)
		if err := unzip(zipPath, dlDir); err != nil {
			c.setStatus("error", fmt.Sprintf("unzip %s: %v", base, err))
			return err
		}
	}

	// Merge
	c.setStatus("merging", "Merging log files…")
	mergedPath := filepath.Join(dlDir, "mail.log")
	cmd := exec.Command("sh", "-c", fmt.Sprintf("cat '%s'/mail.*.log | sort -M -k1 -k2 -k3 > '%s'", dlDir, mergedPath))
	if out, err := cmd.CombinedOutput(); err != nil {
		c.setStatus("error", fmt.Sprintf("merge failed: %v: %s", err, string(out)))
		return err
	}

	// Copy to ~/.mail-pulse/logs/ — merged mail.log + individual pod logs
	logsDir := filepath.Join(home, ".mail-pulse", "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		c.setStatus("error", err.Error())
		return err
	}

	// Copy merged mail.log
	destPath := filepath.Join(logsDir, "mail.log")
	if err := copyFile(mergedPath, destPath); err != nil {
		c.setStatus("error", fmt.Sprintf("copy to logs dir: %v", err))
		return err
	}

	// Copy individual pod log files (mail.0.log, mail.1.log, etc.)
	entries, _ := os.ReadDir(dlDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") || e.Name() == "mail.log" {
			continue
		}
		src := filepath.Join(dlDir, e.Name())
		dst := filepath.Join(logsDir, e.Name())
		_ = copyFile(src, dst)
	}

	// Auto-load
	c.setStatus("loading", "Loading into Mail Pulse…")
	c.logSvc.mu.Lock()
	c.logSvc.selectedFile = "mail.log"
	c.logSvc.ingested = false
	c.logSvc.mu.Unlock()
	c.logSvc.store.clear()
	result := c.logSvc.IngestFile()
	if result["ingested"] != true {
		msg := fmt.Sprintf("ingest: %v", result["status"])
		c.setStatus("error", msg)
		return fmt.Errorf("%s", msg)
	}

	c.setStatus("done", fmt.Sprintf("Loaded %s / %s / %s — %d files merged", sourceName, bucketName, date, len(keys)))
	return nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func unzip(src, dst string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dst, filepath.Base(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
