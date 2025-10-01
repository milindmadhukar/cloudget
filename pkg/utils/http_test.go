package utils

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewHTTPClient(t *testing.T) {
	client := NewHTTPClient()
	if client == nil {
		t.Fatal("NewHTTPClient() returned nil")
	}
	if client.client == nil {
		t.Fatal("HTTPClient.client is nil")
	}
	if client.logger == nil {
		t.Fatal("HTTPClient.logger is nil")
	}
}

func TestHTTPClient_GetFileInfo(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		expectError    bool
		expectFilename string
		expectSize     int64
	}{
		{
			name: "successful file info with content-disposition",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodHead {
						t.Errorf("Expected HEAD request, got %s", r.Method)
					}
					w.Header().Set("Content-Length", "1024")
					w.Header().Set("Content-Disposition", `attachment; filename="test.txt"`)
					w.Header().Set("Accept-Ranges", "bytes")
					w.Header().Set("ETag", `"abc123"`)
					w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 MST")
					w.WriteHeader(http.StatusOK)
				}))
			},
			expectError:    false,
			expectFilename: "test.txt",
			expectSize:     1024,
		},
		{
			name: "file info from URL path",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Length", "2048")
					w.WriteHeader(http.StatusOK)
				}))
			},
			expectError:    false,
			expectFilename: "",
			expectSize:     2048,
		},
		{
			name: "server error",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			client := NewHTTPClient()
			ctx := context.Background()

			fileInfo, err := client.GetFileInfo(ctx, server.URL+"/test.txt", nil)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if fileInfo == nil {
				t.Fatal("fileInfo is nil")
			}

			if tt.expectFilename != "" && fileInfo.Filename != tt.expectFilename {
				t.Errorf("Expected filename %s, got %s", tt.expectFilename, fileInfo.Filename)
			}

			if fileInfo.Size != tt.expectSize {
				t.Errorf("Expected size %d, got %d", tt.expectSize, fileInfo.Size)
			}
		})
	}
}

func TestHTTPClient_DownloadChunk(t *testing.T) {
	testData := "Hello, World! This is test data for chunk download."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if rangeHeader == "" {
			http.Error(w, "Range header required", http.StatusBadRequest)
			return
		}

		// Parse range header (simple implementation for test)
		var start, end int
		if _, err := fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end); err != nil {
			http.Error(w, "Invalid range", http.StatusBadRequest)
			return
		}

		if start >= len(testData) || end >= len(testData) {
			http.Error(w, "Range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}

		chunk := testData[start : end+1]
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(chunk)))
		w.WriteHeader(http.StatusPartialContent)
		w.Write([]byte(chunk))
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	chunk := ChunkInfo{
		Start: 0,
		End:   4,
		Size:  5,
	}

	data, err := client.DownloadChunk(ctx, server.URL, chunk, nil)
	if err != nil {
		t.Fatalf("DownloadChunk failed: %v", err)
	}

	expected := "Hello"
	if string(data) != expected {
		t.Errorf("Expected chunk data %s, got %s", expected, string(data))
	}
}

func TestCalculateChunks(t *testing.T) {
	tests := []struct {
		name        string
		totalSize   int64
		chunkSize   int64
		expectedLen int
		firstChunk  ChunkInfo
		lastChunk   ChunkInfo
	}{
		{
			name:        "exact division",
			totalSize:   100,
			chunkSize:   25,
			expectedLen: 4,
			firstChunk:  ChunkInfo{Start: 0, End: 24, Size: 25},
			lastChunk:   ChunkInfo{Start: 75, End: 99, Size: 25},
		},
		{
			name:        "with remainder",
			totalSize:   103,
			chunkSize:   25,
			expectedLen: 5,
			firstChunk:  ChunkInfo{Start: 0, End: 24, Size: 25},
			lastChunk:   ChunkInfo{Start: 100, End: 102, Size: 3},
		},
		{
			name:        "single chunk",
			totalSize:   10,
			chunkSize:   50,
			expectedLen: 1,
			firstChunk:  ChunkInfo{Start: 0, End: 9, Size: 10},
			lastChunk:   ChunkInfo{Start: 0, End: 9, Size: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunks := calculateChunks(tt.totalSize, tt.chunkSize)

			if len(chunks) != tt.expectedLen {
				t.Errorf("Expected %d chunks, got %d", tt.expectedLen, len(chunks))
			}

			if len(chunks) > 0 {
				if chunks[0] != tt.firstChunk {
					t.Errorf("First chunk = %+v, want %+v", chunks[0], tt.firstChunk)
				}

				if chunks[len(chunks)-1] != tt.lastChunk {
					t.Errorf("Last chunk = %+v, want %+v", chunks[len(chunks)-1], tt.lastChunk)
				}
			}

			// Verify total size matches
			var totalCalculated int64
			for _, chunk := range chunks {
				totalCalculated += chunk.Size
			}
			if totalCalculated != tt.totalSize {
				t.Errorf("Total calculated size %d, want %d", totalCalculated, tt.totalSize)
			}
		})
	}
}

func TestExtractFilename(t *testing.T) {
	tests := []struct {
		name               string
		contentDisposition string
		expected           string
	}{
		{
			name:               "quoted filename",
			contentDisposition: `attachment; filename="document.pdf"`,
			expected:           "document.pdf",
		},
		{
			name:               "unquoted filename",
			contentDisposition: `attachment; filename=document.pdf`,
			expected:           "document.pdf",
		},
		{
			name:               "UTF-8 filename",
			contentDisposition: `attachment; filename*=UTF-8''document%20with%20spaces.pdf`,
			expected:           "document with spaces.pdf",
		},
		{
			name:               "filename with spaces",
			contentDisposition: `attachment; filename="file with spaces.txt"`,
			expected:           "file with spaces.txt",
		},
		{
			name:               "no filename",
			contentDisposition: `attachment`,
			expected:           "",
		},
		{
			name:               "empty string",
			contentDisposition: "",
			expected:           "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFilename(tt.contentDisposition)
			if result != tt.expected {
				t.Errorf("extractFilename(%s) = %s, want %s", tt.contentDisposition, result, tt.expected)
			}
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    int64
		expected string
	}{
		{"Zero bytes", 0, "0 B"},
		{"Bytes", 512, "512 B"},
		{"Kilobytes", 1536, "1.5 KB"},
		{"Megabytes", 1572864, "1.5 MB"},
		{"Gigabytes", 1610612736, "1.5 GB"},
		{"Exact KB", 1024, "1.0 KB"},
		{"Exact MB", 1048576, "1.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			if result != tt.expected {
				t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
			}
		})
	}
}

func TestHTTPClient_DownloadToFile(t *testing.T) {
	testData := "This is test file content for download testing."

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
			w.Header().Set("Accept-Ranges", "bytes")
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == http.MethodGet {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(testData)))
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testData))
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx := context.Background()

	tmpDir := t.TempDir()
	filename := filepath.Join(tmpDir, "download.txt")

	options := &DownloadOptions{
		ChunkSize: 1024,
	}

	err := client.DownloadToFile(ctx, server.URL, filename, options)
	if err != nil {
		t.Fatalf("DownloadToFile failed: %v", err)
	}

	// Verify file was created and has correct content
	content, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("Failed to read downloaded file: %v", err)
	}

	if string(content) != testData {
		t.Errorf("Downloaded content = %s, want %s", string(content), testData)
	}
}

func TestDownloadOptions(t *testing.T) {
	options := &DownloadOptions{
		ChunkSize:  2048,
		MaxRetries: 5,
		RetryDelay: time.Second,
		Headers:    map[string]string{"Authorization": "Bearer token"},
		UserAgent:  "TestAgent/1.0",
		Timeout:    30 * time.Second,
	}

	if options.ChunkSize != 2048 {
		t.Errorf("ChunkSize = %d, want 2048", options.ChunkSize)
	}

	if options.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", options.MaxRetries)
	}

	if options.Headers["Authorization"] != "Bearer token" {
		t.Errorf("Authorization header = %s, want 'Bearer token'", options.Headers["Authorization"])
	}
}

func TestHTTPClientWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewHTTPClient()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GetFileInfo(ctx, server.URL, nil)
	if err == nil {
		t.Error("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}
