package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sirupsen/logrus"
)

type HTTPClient struct {
	client *resty.Client
	logger *logrus.Logger
}

type ChunkInfo struct {
	Start int64
	End   int64
	Size  int64
}

type DownloadOptions struct {
	ChunkSize    int64
	MaxRetries   int
	RetryDelay   time.Duration
	Headers      map[string]string
	UserAgent    string
	Timeout      time.Duration
	ProgressFunc func(downloaded, total int64)
}

func NewHTTPClient() *HTTPClient {
	client := resty.New()
	client.SetTimeout(30 * time.Second)
	client.SetRetryCount(3)
	client.SetRetryWaitTime(2 * time.Second)
	client.SetRetryMaxWaitTime(10 * time.Second)
	client.SetHeader("User-Agent", "Go-Downloader/1.0")

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &HTTPClient{
		client: client,
		logger: logger,
	}
}

func (h *HTTPClient) SetLogger(logger *logrus.Logger) {
	h.logger = logger
}

func (h *HTTPClient) GetFileInfo(ctx context.Context, urlStr string, headers map[string]string) (*FileInfo, error) {
	req := h.client.R().SetContext(ctx)

	if headers != nil {
		req.SetHeaders(headers)
	}

	resp, err := req.Head(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	if resp.StatusCode() != http.StatusOK && resp.StatusCode() != http.StatusPartialContent {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	fileInfo := &FileInfo{
		URL: urlStr,
	}

	if contentLength := resp.Header().Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			fileInfo.Size = size
		}
	}

	if contentDisposition := resp.Header().Get("Content-Disposition"); contentDisposition != "" {
		if filename := extractFilename(contentDisposition); filename != "" {
			fileInfo.Filename = filename
		}
	}

	if fileInfo.Filename == "" {
		if parsedURL, err := url.Parse(fileInfo.URL); err == nil {
			fileInfo.Filename = path.Base(parsedURL.Path)
			if fileInfo.Filename == "" || fileInfo.Filename == "/" || fileInfo.Filename == "." {
				fileInfo.Filename = "download"
			}
		}
	}

	fileInfo.SupportsRangeRequests = resp.Header().Get("Accept-Ranges") == "bytes"

	if etag := resp.Header().Get("ETag"); etag != "" {
		fileInfo.ETag = strings.Trim(etag, `"`)
	}

	if lastModified := resp.Header().Get("Last-Modified"); lastModified != "" {
		if t, err := time.Parse(time.RFC1123, lastModified); err == nil {
			fileInfo.LastModified = &t
		}
	}

	return fileInfo, nil
}

func (h *HTTPClient) DownloadChunk(ctx context.Context, urlStr string, chunk ChunkInfo, options *DownloadOptions) ([]byte, error) {
	req := h.client.R().SetContext(ctx)

	if options != nil && options.Headers != nil {
		req.SetHeaders(options.Headers)
	}

	rangeHeader := fmt.Sprintf("bytes=%d-%d", chunk.Start, chunk.End)
	req.SetHeader("Range", rangeHeader)

	maxRetries := 3
	retryDelay := 2 * time.Second
	if options != nil {
		if options.MaxRetries > 0 {
			maxRetries = options.MaxRetries
		}
		if options.RetryDelay > 0 {
			retryDelay = options.RetryDelay
		}
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			h.logger.Warnf("Retrying chunk download (attempt %d/%d) for range %d-%d",
				attempt, maxRetries, chunk.Start, chunk.End)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryDelay):
			}
		}

		resp, err := req.Get(urlStr)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		if resp.StatusCode() != http.StatusPartialContent && resp.StatusCode() != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode())
			continue
		}

		body := resp.Body()
		if int64(len(body)) != chunk.Size {
			lastErr = fmt.Errorf("received %d bytes, expected %d bytes", len(body), chunk.Size)
			continue
		}

		return body, nil
	}

	return nil, fmt.Errorf("failed to download chunk after %d attempts: %w", maxRetries+1, lastErr)
}

func (h *HTTPClient) DownloadToFile(ctx context.Context, urlStr, filename string, options *DownloadOptions) error {
	fileInfo, err := h.GetFileInfo(ctx, urlStr, options.Headers)
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	if fileInfo.Size == 0 {
		return h.downloadSimple(ctx, urlStr, filename, options)
	}

	if !fileInfo.SupportsRangeRequests {
		h.logger.Warn("Server doesn't support range requests, falling back to simple download")
		return h.downloadSimple(ctx, urlStr, filename, options)
	}

	chunkSize := int64(1024 * 1024) // 1MB default
	if options != nil && options.ChunkSize > 0 {
		chunkSize = options.ChunkSize
	}

	return h.downloadChunked(ctx, urlStr, filename, fileInfo.Size, chunkSize, options)
}

func (h *HTTPClient) downloadSimple(ctx context.Context, urlStr, filename string, options *DownloadOptions) error {
	req := h.client.R().SetContext(ctx)

	if options != nil && options.Headers != nil {
		req.SetHeaders(options.Headers)
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	resp, err := req.SetOutput(filename).Get(urlStr)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}

	return nil
}

func (h *HTTPClient) downloadChunked(ctx context.Context, urlStr, filename string, totalSize, chunkSize int64, options *DownloadOptions) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	chunks := calculateChunks(totalSize, chunkSize)

	// Download chunks sequentially for now
	// TODO: Implement parallel downloading with worker pool
	var downloaded int64
	for _, chunk := range chunks {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		data, err := h.DownloadChunk(ctx, urlStr, chunk, options)
		if err != nil {
			return fmt.Errorf("failed to download chunk %d-%d: %w", chunk.Start, chunk.End, err)
		}

		if _, err := file.WriteAt(data, chunk.Start); err != nil {
			return fmt.Errorf("failed to write chunk to file: %w", err)
		}

		downloaded += chunk.Size
		if options != nil && options.ProgressFunc != nil {
			options.ProgressFunc(downloaded, totalSize)
		}
	}

	return nil
}

func calculateChunks(totalSize, chunkSize int64) []ChunkInfo {
	var chunks []ChunkInfo

	for start := int64(0); start < totalSize; start += chunkSize {
		end := start + chunkSize - 1
		if end >= totalSize {
			end = totalSize - 1
		}

		chunks = append(chunks, ChunkInfo{
			Start: start,
			End:   end,
			Size:  end - start + 1,
		})
	}

	return chunks
}

func extractFilename(contentDisposition string) string {
	// Try to extract filename from Content-Disposition header
	// Format: attachment; filename="filename.ext" or filename*=UTF-8''filename.ext

	// First try the standard filename parameter
	re := regexp.MustCompile(`filename="?([^";\r\n]+)"?`)
	matches := re.FindStringSubmatch(contentDisposition)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// Try filename* for UTF-8 encoded filenames
	re = regexp.MustCompile(`filename\*=UTF-8''([^;\r\n]+)`)
	matches = re.FindStringSubmatch(contentDisposition)
	if len(matches) > 1 {
		// URL decode the filename
		if decoded, err := url.QueryUnescape(matches[1]); err == nil {
			return decoded
		}
		return matches[1]
	}

	return ""
}

// FileInfo represents information about a downloadable file
type FileInfo struct {
	URL                   string
	Filename              string
	Size                  int64
	ETag                  string
	LastModified          *time.Time
	SupportsRangeRequests bool
}

// FormatBytes formats bytes for display
func FormatBytes(bytes int64) string {
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
