package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloud-downloader/downloader/pkg/downloader"
	"github.com/cloud-downloader/downloader/pkg/interfaces"
	"github.com/sirupsen/logrus"
)

var (
	url            = flag.String("url", "", "URL to download")
	urls           = flag.String("urls", "", "Comma-separated list of URLs to download")
	urlFile        = flag.String("url-file", "", "File containing URLs to download (one per line)")
	outputDir      = flag.String("output-dir", ".", "Output directory for downloads")
	outputPath     = flag.String("output", "", "Specific output file path (for single URL)")
	filename       = flag.String("filename", "", "Custom filename (for single URL)")
	maxConnections = flag.Int("max-connections", 8, "Maximum concurrent connections per download")
	chunkSize      = flag.String("chunk-size", "2MB", "Chunk size for downloads (e.g., 1MB, 512KB)")
	timeout        = flag.Duration("timeout", 300*time.Second, "Download timeout")
	resume         = flag.Bool("resume", true, "Enable download resume")
	verifyHash     = flag.String("verify-hash", "", "Expected hash for verification")
	hashAlgorithm  = flag.String("hash-algorithm", "sha256", "Hash algorithm (md5, sha1, sha256, sha512)")
	verbose        = flag.Bool("verbose", false, "Enable verbose logging")
	quiet          = flag.Bool("quiet", false, "Suppress all output except errors")
	showProgress   = flag.Bool("progress", true, "Show download progress")
	showHelp       = flag.Bool("help", false, "Show help message")
)

func main() {
	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	// Setup logging
	logger := logrus.New()
	if *quiet {
		logger.SetLevel(logrus.ErrorLevel)
	} else if *verbose {
		logger.SetLevel(logrus.DebugLevel)
	} else {
		logger.SetLevel(logrus.InfoLevel)
	}

	// Parse chunk size
	chunkSizeBytes, err := parseSize(*chunkSize)
	if err != nil {
		logger.Fatalf("Invalid chunk size: %v", err)
	}

	// Collect URLs to download
	urlList, err := collectURLs()
	if err != nil {
		logger.Fatalf("Error collecting URLs: %v", err)
	}

	if len(urlList) == 0 {
		logger.Fatal("No URLs provided. Use -url, -urls, or -url-file to specify URLs to download.")
	}

	// Create download manager
	manager := downloader.NewManager(&downloader.ManagerOptions{
		MaxConnections: *maxConnections,
		ChunkSize:      chunkSizeBytes,
		Timeout:        *timeout,
		OutputDir:      *outputDir,
		Resume:         *resume,
		VerifyHash:     *verifyHash != "",
		HashAlgorithm:  *hashAlgorithm,
	})

	manager.SetLogger(logger)

	// Download all URLs
	ctx := context.Background()
	overallStart := time.Now()
	var totalBytes int64
	var successCount, failCount int

	for i, downloadURL := range urlList {
		logger.Infof("Downloading %d/%d: %s", i+1, len(urlList), downloadURL)

		// Create download request
		req := &interfaces.DownloadRequest{
			URL:            downloadURL,
			OutputPath:     *outputPath,
			CustomFilename: *filename,
			VerifyHash:     *verifyHash,
		}

		// Perform download
		result, err := manager.Download(ctx, req)
		if err != nil {
			logger.Errorf("Download failed: %v", err)
			failCount++
			continue
		}

		// Show results
		logger.Infof("Download completed successfully!")
		logger.Infof("File: %s", result.FilePath)
		logger.Infof("Size: %s", formatBytes(result.Size))
		logger.Infof("Time: %.1f seconds", result.Duration.Seconds())
		logger.Infof("Speed: %.1f MB/s", result.Speed)

		if result.Hash != "" {
			logger.Infof("Hash (%s): %s", *hashAlgorithm, result.Hash)
		}

		totalBytes += result.Size
		successCount++
		fmt.Println() // Empty line between downloads
	}

	// Show overall summary
	overallDuration := time.Since(overallStart)
	overallSpeed := float64(totalBytes) / overallDuration.Seconds() / 1024 / 1024 // MB/s

	logger.Infof("=== Download Summary ===")
	logger.Infof("Total URLs: %d", len(urlList))
	logger.Infof("Successful: %d", successCount)
	logger.Infof("Failed: %d", failCount)
	logger.Infof("Total size: %s", formatBytes(totalBytes))
	logger.Infof("Total time: %.1f seconds", overallDuration.Seconds())
	logger.Infof("Overall speed: %.1f MB/s", overallSpeed)

	if failCount > 0 {
		os.Exit(1)
	}
}

func collectURLs() ([]string, error) {
	var urlList []string

	// Single URL
	if *url != "" {
		urlList = append(urlList, *url)
	}

	// Multiple URLs (comma-separated)
	if *urls != "" {
		multipleURLs := strings.Split(*urls, ",")
		for _, u := range multipleURLs {
			u = strings.TrimSpace(u)
			if u != "" {
				urlList = append(urlList, u)
			}
		}
	}

	// URLs from file
	if *urlFile != "" {
		fileURLs, err := readURLsFromFile(*urlFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read URLs from file: %w", err)
		}
		urlList = append(urlList, fileURLs...)
	}

	return urlList, nil
}

func readURLsFromFile(filename string) ([]string, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var urls []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}

	return urls, nil
}

func parseSize(sizeStr string) (int64, error) {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// Extract number and unit
	var num string
	var unit string
	for i, r := range sizeStr {
		if r >= '0' && r <= '9' || r == '.' {
			num += string(r)
		} else {
			unit = sizeStr[i:]
			break
		}
	}

	if num == "" {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	size, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in size: %s", num)
	}

	multiplier := int64(1)
	switch unit {
	case "", "B":
		multiplier = 1
	case "KB":
		multiplier = 1024
	case "MB":
		multiplier = 1024 * 1024
	case "GB":
		multiplier = 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown size unit: %s", unit)
	}

	return int64(size * float64(multiplier)), nil
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func printHelp() {
	fmt.Printf(`Cloud Downloader CLI - Download files from Dropbox, Google Drive, and WeTransfer

Usage:
  %s -url "https://example.com/file" [options]
  %s -urls "url1,url2,url3" [options] 
  %s -url-file urls.txt [options]

Examples:
  # Download a single file
  %s -url "https://dropbox.com/s/abc123/file.zip"
  
  # Download multiple files
  %s -urls "https://dropbox.com/s/abc/file1.zip,https://drive.google.com/file/d/xyz/view"
  
  # Download from file list
  %s -url-file urls.txt -output-dir ./downloads
  
  # Download with custom settings
  %s -url "https://we.tl/t-abc123" -chunk-size 5MB -max-connections 16

Options:
`, os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0], os.Args[0])

	flag.PrintDefaults()

	fmt.Printf(`
Supported Services:
  - Dropbox (dropbox.com/s/, dropbox.com/scl/fi/)
  - Google Drive (drive.google.com, docs.google.com)
  - WeTransfer (we.tl, wetransfer.com)

Notes:
  - URLs are automatically converted to direct download links
  - Downloads support resume functionality by default
  - Large files are downloaded in chunks for better performance
  - Hash verification is optional but recommended for important files
`)
}
