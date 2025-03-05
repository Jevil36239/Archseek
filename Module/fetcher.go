package module

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
	"gopkg.in/ini.v1"

    "archseek/loader"
    
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

const (
	WaybackURL = "https://web.archive.org/cdx/search/cdx"
)

var (
	FileExtensions string
)

var (
	cyan    = color.New(color.FgCyan)
	red     = color.New(color.FgRed)
	green   = color.New(color.FgGreen)
	magenta = color.New(color.FgMagenta)
)

type WaybackResponse struct {
	URLs []string
}

func FetchWaybackURLs(domain string) []string {
    loader := loader.New("[INFO] Fetching URLs from Wayback Machine")
    loader.Start()
    defer loader.Stop()

    params := url.Values{}
    params.Add("url", "*."+domain+"/*")
    params.Add("collapse", "urlkey")
    params.Add("output", "text")
    params.Add("fl", "original")

    resp, err := http.Get(WaybackURL + "?" + params.Encode())
    if err != nil {
        red.Print("[ERROR] ")
        fmt.Printf("Failed to fetch URLs from Wayback Machine for %s\n", domain)
        return nil
    }
    defer resp.Body.Close()

    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return nil
    }

    urls := strings.Split(string(body), "\n")
    // Remove empty strings
    var filteredUrls []string
    for _, url := range urls {
        if url != "" {
            filteredUrls = append(filteredUrls, url)
        }
    }

    green.Print("[SUCCESS] ")
    red.Printf("%d ", len(filteredUrls))
    fmt.Printf("URLs retrieved from Wayback Machine for %s\n", domain)
    loader.Stop()

    return filteredUrls
}

func FilterURLsByFiletype(urls []string) []string {
	// Load file extensions from settings.ini
	cfg, err := ini.Load("settings.ini")
	if err != nil {
		red.Print("[ERROR] ")
		fmt.Printf("Failed to load settings: %v\n", err)
		return nil
	}
	FileExtensions = cfg.Section("FileExtensions").Key("Extensions").MustString(`\.(xls|xml|xlsx|json|pdf|sql|doc|docx|pptx|txt|zip|tar\.gz|tgz|bak|7z|rar|log|cache|secret|db|backup|yml|gz|config|csv|yaml|md|md5|exe|dll|bin|ini|bat|sh|tar|deb|rpm|iso|img|apk|msi|dmg|tmp|crt|pem|key|pub|asc)`)
	regex := regexp.MustCompile(FileExtensions)
	filtered := make([]string, 0)

	// Create progress bar for filtering
	bar := progressbar.NewOptions(len(urls),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowCount(),
		progressbar.OptionSetWidth(15),
		progressbar.OptionSetDescription("[cyan]Filtering URLs..."),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[cyan][",
			BarEnd:        "][reset]",
		}),
	)

	for _, u := range urls {
		if regex.MatchString(strings.ToLower(u)) {
			filtered = append(filtered, u)
		}
		bar.Add(1)
	}

	cyan.Print("[INFO] ")
	red.Printf("%d ", len(filtered))
	fmt.Println("URLs matching file types")

	return filtered
}

func ValidateURLs(urls []string) []string {
	var validURLs []string
	var mu sync.Mutex

    // Load settings from ini file
    cfg, err := ini.Load("settings.ini")
    if err != nil {
        red.Print("[ERROR] ")
        fmt.Printf("Failed to load settings: %v\n", err)
        return nil
    }

    batchSize := cfg.Section("BatchProcessing").Key("BatchSize").MustInt(10)
    maxThreads := cfg.Section("BatchProcessing").Key("MaxThreads").MustInt(5)
    timeout := cfg.Section("BatchProcessing").Key("Timeout").MustInt(15)

    // Create download manager with configured max threads
    dm := NewDownloadManager(maxThreads)
    if err := os.MkdirAll(dm.OutputDir, 0755); err != nil {
        red.Print("[ERROR] ")
        fmt.Printf("Failed to create download directory: %v\n", err)
        return validURLs
    }

    // Process URLs in batches
    totalBatches := (len(urls) + batchSize - 1) / batchSize

    // Create progress bar for batch processing
    batchLoader := loader.New("[INFO] Processing batches")
    batchLoader.Start()
    defer batchLoader.Stop()

    bar := progressbar.NewOptions(totalBatches,
        progressbar.OptionEnableColorCodes(true),
        progressbar.OptionShowCount(),
        progressbar.OptionSetWidth(15),
        progressbar.OptionSetDescription(batchLoader.Color().Sprint(batchLoader.CurrentFrame()) + " [cyan]Processing batches..."),
        progressbar.OptionSetTheme(progressbar.Theme{
            Saucer:        "[green]=[reset]",
            SaucerHead:    "[green]>[reset]",
            SaucerPadding: " ",
            BarStart:      "[cyan][",
            BarEnd:        "][reset]",
        }),
    )

    // Create HTTP client with configured timeout
    client := &http.Client{
        Timeout: time.Duration(timeout) * time.Second,
        Transport: &http.Transport{
            MaxIdleConns:        50,
            MaxIdleConnsPerHost: 50,
            IdleConnTimeout:     90 * time.Second,
            DisableCompression: true,
        },
    }

    // Process URLs in batches
    for i := 0; i < len(urls); i += batchSize {
        end := i + batchSize
        if end > len(urls) {
            end = len(urls)
        }
        batch := urls[i:end]

        // Validate URLs in current batch
        batchValidURLs := make([]string, 0)
        var batchWg sync.WaitGroup
        batchResults := make(chan string, len(batch))

        for _, url := range batch {
            batchWg.Add(1)
            go func(url string) {
                defer batchWg.Done()
                maxRetries := 3

                for attempt := 1; attempt <= maxRetries; attempt++ {
                    resp, err := client.Head(url)
                    if err != nil {
                        if attempt == maxRetries {
                            red.Print("[ERROR] ")
                            fmt.Printf("Failed to validate %s after %d attempts: %v\n", url, maxRetries, err)
                        }
                        time.Sleep(time.Duration(attempt) * time.Second)
                        continue
                    }
                    resp.Body.Close()

                    if resp.StatusCode == 200 {
                        batchResults <- url
                        break
                    }

                    if attempt == maxRetries {
                        red.Print("[ERROR] ")
                        fmt.Printf("URL %s returned status code %d\n", url, resp.StatusCode)
                    }
                    break
                }
            }(url)
        }

        // Wait for batch validation to complete
        go func() {
            batchWg.Wait()
            close(batchResults)
        }()

        // Collect valid URLs from batch
        for url := range batchResults {
            batchValidURLs = append(batchValidURLs, url)
        }

        // Download valid URLs from the batch
        if len(batchValidURLs) > 0 {
            if err := dm.Download(batchValidURLs); err != nil {
                red.Print("[ERROR] ")
                fmt.Printf("Error during batch download: %v\n", err)
            }
        }

        // Update overall valid URLs list
        mu.Lock()
        validURLs = append(validURLs, batchValidURLs...)
        mu.Unlock()

        bar.Add(1)
        cyan.Print("[INFO] ")
        fmt.Printf("Batch %d/%d: Found %d valid URLs\n", (i/batchSize)+1, totalBatches, len(batchValidURLs))
    }

    green.Print("[SUCCESS] ")
    red.Printf("%d ", len(validURLs))
    fmt.Println("valid URLs processed")

    return validURLs
}

func SaveToFile(data []string, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, url := range data {
		_, err := writer.WriteString(url + "\n")
		if err != nil {
			return err
		}
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	cyan.Print("[INFO] ")
	fmt.Printf("Saved valid URLs to %s\n", filename)
	return nil
}

func ProcessDomains(domains []string) {
    var allURLs []string

    for _, domain := range domains {
        fmt.Print("\n")
        cyan.Print("[INFO] ")
        fmt.Printf("Processing domain: %s\n", domain)

        waybackURLs := FetchWaybackURLs(domain)
        filteredURLs := FilterURLsByFiletype(waybackURLs)
        allURLs = append(allURLs, filteredURLs...)
    }

    validURLs := ValidateURLs(allURLs)
    err := SaveToFile(validURLs, "valid_urls.txt")
    if err != nil {
        red.Print("[ERROR] ")
        fmt.Printf("Failed to save URLs: %v\n", err)
        return
    }

    magenta.Print("[RESULT] ")
    fmt.Print("Total valid URLs: ")
    red.Printf("%d\n", len(validURLs))
}