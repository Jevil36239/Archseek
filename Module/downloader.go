package module

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"strings"
	"time"

    "archseek/loader"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

type FileMetadata struct {
	URL      string
	Size     int64
	Filename string
	Path     string
}

type DownloadManager struct {
    Concurrency int
    OutputDir   string
    Metadata    []FileMetadata
    mu          sync.Mutex
    totalBytes  int64
    downloaded  int64
    failedURLs  []string
    maxRetries  int
    retryDelay  time.Duration
}

func NewDownloadManager(concurrency int) *DownloadManager {
    return &DownloadManager{
        Concurrency: concurrency,
        OutputDir:   "downloads",
        Metadata:    make([]FileMetadata, 0),
        failedURLs:  make([]string, 0),
        maxRetries:  5,                    // Maximum number of retries
        retryDelay:  2 * time.Second,      // Base delay between retries
    }
}

func (dm *DownloadManager) Download(urls []string) error {
    if err := os.MkdirAll(dm.OutputDir, 0755); err != nil {
        return err
    }

    // Process failed downloads from previous runs
    failedLogPath := filepath.Join(dm.OutputDir, "failed_downloads.log")
    if _, err := os.Stat(failedLogPath); err == nil {
        if content, err := os.ReadFile(failedLogPath); err == nil {
            previousFailures := strings.Split(string(content), "\n")
            for _, url := range previousFailures {
                if url = strings.TrimSpace(url); url != "" {
                    urls = append(urls, url)
                }
            }
        }
        // Remove the old failed downloads log
        os.Remove(failedLogPath)
    }

    // Create job and result queues with limited buffer
    jobQueue := make(chan string, dm.Concurrency*2)
    var wg sync.WaitGroup

    downloadLoader := loader.New("[INFO] Downloading files")
    downloadLoader.Start()
    defer downloadLoader.Stop()

    // Create progress bar for overall progress
    bar := progressbar.NewOptions(len(urls),
        progressbar.OptionEnableColorCodes(true),
        progressbar.OptionShowBytes(true),
        progressbar.OptionSetWidth(15),
        progressbar.OptionSetDescription(downloadLoader.Color().Sprint(downloadLoader.CurrentFrame()) + " [cyan]Downloading files..."),
        progressbar.OptionSetTheme(progressbar.Theme{
            Saucer:        "[green]=[reset]",
            SaucerHead:    "[green]>[reset]",
            SaucerPadding: " ",
            BarStart:      "[cyan][",
            BarEnd:        "][reset]",
        }),
    )

    // Start worker pool with controlled concurrency
    for i := 0; i < dm.Concurrency; i++ {
        wg.Add(1)
        go func(workerID int) {
            defer wg.Done()
            rateLimiter := time.NewTicker(500 * time.Millisecond)
            defer rateLimiter.Stop()

            for url := range jobQueue {
                <-rateLimiter.C
                if err := dm.downloadFile(url); err != nil {
                    dm.mu.Lock()
                    dm.failedURLs = append(dm.failedURLs, url)
                    dm.mu.Unlock()
                    red.Print("[ERROR] ")
                    fmt.Printf("Worker %d: Failed to download %s: %v\n", workerID, url, err)
                }
                bar.Add(1)
                time.Sleep(200 * time.Millisecond)
            }
        }(i)
    }

    // Feed URLs to job queue with rate limiting
    urlFeeder := time.NewTicker(100 * time.Millisecond)
    defer urlFeeder.Stop()

    for _, url := range urls {
        <-urlFeeder.C // Rate limit URL feeding
        jobQueue <- url
    }
    close(jobQueue)

    wg.Wait()
    bar.Finish()

    // Save failed URLs to a log file and retry if needed
    if len(dm.failedURLs) > 0 {
        logFile := filepath.Join(dm.OutputDir, "failed_downloads.log")
        if err := os.WriteFile(logFile, []byte(strings.Join(dm.failedURLs, "\n")), 0644); err != nil {
            fmt.Printf("Failed to save failed downloads log: %v\n", err)
        } else {
            red.Print("[WARNING] ")
            fmt.Printf("%d downloads failed. See %s for details.\n", len(dm.failedURLs), logFile)
        }
    }

    return nil
}

func (dm *DownloadManager) downloadFile(fileURL string) error {
    var lastErr error

    for attempt := 0; attempt < dm.maxRetries; attempt++ {
        if attempt > 0 {
            // Exponential backoff with jitter
            backoff := dm.retryDelay * time.Duration(1<<uint(attempt))
            jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
            time.Sleep(backoff + jitter)
        }

        client := &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 100,
                IdleConnTimeout:     90 * time.Second,
                DisableCompression:  true,
            },
        }

        resp, err := client.Get(fileURL)
        if err != nil {
            lastErr = fmt.Errorf("attempt %d: %v", attempt+1, err)
            continue
        }

        if resp.StatusCode != http.StatusOK {
            resp.Body.Close()
            lastErr = fmt.Errorf("attempt %d: bad status: %s", attempt+1, resp.Status)
            // Don't retry on permanent errors
            if resp.StatusCode == http.StatusNotFound ||
               resp.StatusCode == http.StatusForbidden ||
               resp.StatusCode == http.StatusUnauthorized {
                return lastErr
            }
            continue
        }

        defer resp.Body.Close()

        if resp.StatusCode != http.StatusOK {
            lastErr = fmt.Errorf("bad status: %s", resp.Status)
            if attempt == dm.maxRetries {
                return lastErr
            }
            continue
        }

        // Extract domain and create subdirectory
        parsedURL, err := url.Parse(fileURL)
        if err != nil {
            return err
        }

        domainDir := filepath.Join(dm.OutputDir, parsedURL.Host)
        if err := os.MkdirAll(domainDir, 0755); err != nil {
            return err
        }

        // Generate filename from URL
        filename := filepath.Base(parsedURL.Path)
        if filename == "/" || filename == "" {
            filename = "index.html"
        }

        filePath := filepath.Join(domainDir, filename)

        // Create the file
        file, err := os.Create(filePath)
        if err != nil {
            return err
        }
        defer file.Close()

        // Create progress bar for individual file
        bar := progressbar.NewOptions64(
            resp.ContentLength,
            progressbar.OptionEnableColorCodes(true),
            progressbar.OptionShowBytes(true),
            progressbar.OptionSetWidth(15),
            progressbar.OptionSetDescription(fmt.Sprintf("[cyan]%s", filename)),
            progressbar.OptionSetTheme(progressbar.Theme{
                Saucer:        "[green]=[reset]",
                SaucerHead:    "[green]>[reset]",
                SaucerPadding: " ",
                BarStart:      "[cyan][",
                BarEnd:        "][reset]",
            }),
        )

        // Copy the body to file with progress
        size, err := io.Copy(io.MultiWriter(file, bar), resp.Body)
        if err != nil {
            // If copy fails, try to remove the partially downloaded file
            os.Remove(filePath)
            lastErr = err
            if attempt == dm.maxRetries {
                return fmt.Errorf("failed to download after %d attempts: %v", dm.maxRetries, err)
            }
            continue
        }

        // Update total downloaded bytes
        atomic.AddInt64(&dm.downloaded, size)

        // Store metadata
        dm.mu.Lock()
        dm.Metadata = append(dm.Metadata, FileMetadata{
            URL:      fileURL,
            Size:     size,
            Filename: filename,
            Path:     filePath,
        })
        dm.mu.Unlock()

        // Print download information
        green.Print("[SUCCESS] ")
        fmt.Printf("Downloaded: %s\n", filename)
        fmt.Printf("  Local Path: %s\n", filePath)
        color.New(color.FgHiBlack).Printf("  Web Path: %s\n", fileURL)
        fmt.Printf("  Size: %s\n", humanize.Bytes(uint64(size)))
        fmt.Printf("  Type: %s\n", resp.Header.Get("Content-Type"))

        return nil // Success, exit retry loop
    }

    return lastErr // Should never reach here due to returns in loop
}

func (dm *DownloadManager) GetMetadata() []FileMetadata {
	dm.mu.Lock()
	defer dm.mu.Unlock()
	return dm.Metadata
}