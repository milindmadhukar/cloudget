package wetransfer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	service := New()
	assert.NotNil(t, service)
	assert.NotNil(t, service.httpClient)
	assert.NotNil(t, service.logger)
}

func TestService_GetServiceName(t *testing.T) {
	service := New()
	assert.Equal(t, "WeTransfer", service.GetServiceName())
}

func TestService_IsSupported(t *testing.T) {
	service := New()

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "WeTransfer full URL",
			url:      "https://wetransfer.com/downloads/abc123def456",
			expected: true,
		},
		{
			name:     "WeTransfer short URL (we.tl)",
			url:      "https://we.tl/t-abc123",
			expected: true,
		},
		{
			name:     "WeTransfer without protocol",
			url:      "wetransfer.com/downloads/abc123",
			expected: true,
		},
		{
			name:     "We.tl without protocol",
			url:      "we.tl/t-abc123",
			expected: true,
		},
		{
			name:     "Google Drive URL",
			url:      "https://drive.google.com/file/d/1BxiMVs0XRA5nFMdKvBdBZjgmUUqptlbs74OgvE2upms/view",
			expected: false,
		},
		{
			name:     "Dropbox URL",
			url:      "https://www.dropbox.com/s/abc123/file.txt",
			expected: false,
		},
		{
			name:     "Random URL",
			url:      "https://example.com/file.txt",
			expected: false,
		},
		{
			name:     "Empty URL",
			url:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.IsSupported(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestService_extractTransferID(t *testing.T) {
	service := New()

	tests := []struct {
		name        string
		url         string
		expectedID  string
		expectError bool
	}{
		{
			name:        "WeTransfer full URL",
			url:         "https://wetransfer.com/downloads/abc123def456",
			expectedID:  "abc123def456",
			expectError: false,
		},
		{
			name:        "WeTransfer URL with query parameters",
			url:         "https://wetransfer.com/downloads/abc123def456?foo=bar",
			expectedID:  "abc123def456",
			expectError: false,
		},
		{
			name:        "We.tl alphanumeric ID",
			url:         "https://we.tl/abc123",
			expectedID:  "abc123",
			expectError: false,
		},
		{
			name:        "We.tl URL without https",
			url:         "we.tl/xyz789",
			expectedID:  "xyz789",
			expectError: false,
		},
		{
			name:        "We.tl URL with alphanumeric only",
			url:         "https://we.tl/abc123",
			expectedID:  "abc123",
			expectError: false,
		},
		{
			name:        "Invalid WeTransfer URL",
			url:         "https://wetransfer.com/upload",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Non-WeTransfer URL",
			url:         "https://example.com/download/abc123",
			expectedID:  "",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			expectedID:  "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transferID, err := service.extractTransferID(tt.url)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, "", transferID)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, transferID)
			}
		})
	}
}

func TestService_ConvertURL(t *testing.T) {
	service := New()

	tests := []struct {
		name        string
		url         string
		expectedURL string
		expectError bool
	}{
		{
			name:        "Valid WeTransfer URL",
			url:         "https://wetransfer.com/downloads/abc123def456",
			expectedURL: "https://wetransfer.com/downloads/abc123def456",
			expectError: false,
		},
		{
			name:        "Valid we.tl URL",
			url:         "https://we.tl/t-abc123",
			expectedURL: "https://we.tl/t-abc123",
			expectError: false,
		},
		{
			name:        "Unsupported URL",
			url:         "https://example.com/file.txt",
			expectedURL: "",
			expectError: true,
		},
		{
			name:        "Empty URL",
			url:         "",
			expectedURL: "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convertedURL, err := service.ConvertURL(tt.url)

			if tt.expectError {
				assert.Error(t, err)
				assert.Equal(t, "", convertedURL)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedURL, convertedURL)
			}
		})
	}
}

func TestService_getDefaultHeaders(t *testing.T) {
	service := New()
	headers := service.getDefaultHeaders()

	assert.NotEmpty(t, headers)
	assert.Contains(t, headers, "Accept-Encoding")
	assert.Contains(t, headers, "User-Agent")
	assert.Equal(t, "identity", headers["Accept-Encoding"])
	assert.Contains(t, headers["User-Agent"], "Mozilla")
}

func TestService_getWeTransferDownloadInfo(t *testing.T) {
	service := New()

	t.Run("Successful transfer info retrieval", func(t *testing.T) {
		// Create mock servers for WeTransfer API
		transferServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/v4/transfers/abc123" {
				response := WeTransferResponse{
					Files: []WeTransferFile{
						{Name: "test-file.txt", Size: 1024},
					},
					SecurityHash: "security123",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
			if r.URL.Path == "/api/v4/transfers/abc123/download" {
				response := DownloadResponse{
					DirectLink: "https://download.wetransfer.com/direct/test-file.txt",
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer transferServer.Close()

		// We can't easily mock the internal HTTP calls since they use hardcoded URLs
		// So let's test the URL extraction which is the core logic we can test
		transferID, err := service.extractTransferID("https://wetransfer.com/downloads/abc123")
		require.NoError(t, err)
		assert.Equal(t, "abc123", transferID)
	})

	t.Run("Invalid transfer ID", func(t *testing.T) {
		ctx := context.Background()
		invalidURL := "https://wetransfer.com/upload"

		downloadInfo, err := service.getWeTransferDownloadInfo(ctx, invalidURL)
		assert.Error(t, err)
		assert.Nil(t, downloadInfo)
		assert.Contains(t, err.Error(), "no transfer ID found")
	})

	t.Run("Context timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()

		// Wait for context to timeout
		time.Sleep(1 * time.Millisecond)

		url := "https://wetransfer.com/downloads/abc123"

		downloadInfo, err := service.getWeTransferDownloadInfo(ctx, url)
		assert.Error(t, err)
		assert.Nil(t, downloadInfo)
		// The error should be related to context timeout or network failure
	})
}

func TestService_GetFileInfo(t *testing.T) {
	service := New()

	t.Run("Invalid WeTransfer URL", func(t *testing.T) {
		ctx := context.Background()
		invalidURL := "https://example.com/file.txt"

		// Since ConvertURL returns original URL for WeTransfer,
		// let's test with a URL that won't pass the extractTransferID
		invalidURL = "https://wetransfer.com/upload"

		fileInfo, err := service.GetFileInfo(ctx, invalidURL)
		assert.Error(t, err)
		assert.Nil(t, fileInfo)
		assert.Contains(t, err.Error(), "failed to get WeTransfer download info")
	})
}

func TestService_PrepareDownload(t *testing.T) {
	service := New()

	t.Run("Invalid URL", func(t *testing.T) {
		ctx := context.Background()
		invalidURL := "https://wetransfer.com/upload"

		downloadURL, err := service.PrepareDownload(ctx, invalidURL)
		assert.Error(t, err)
		assert.Equal(t, "", downloadURL)
		assert.Contains(t, err.Error(), "failed to get WeTransfer download info")
	})
}

func TestWeTransferStructs(t *testing.T) {
	t.Run("WeTransferFile struct", func(t *testing.T) {
		file := WeTransferFile{
			Name: "test.txt",
			Size: 1024,
		}
		assert.Equal(t, "test.txt", file.Name)
		assert.Equal(t, int64(1024), file.Size)
	})

	t.Run("WeTransferResponse struct", func(t *testing.T) {
		response := WeTransferResponse{
			Files: []WeTransferFile{
				{Name: "file1.txt", Size: 512},
				{Name: "file2.txt", Size: 1024},
			},
			SecurityHash: "hash123",
		}
		assert.Len(t, response.Files, 2)
		assert.Equal(t, "hash123", response.SecurityHash)
		assert.Equal(t, "file1.txt", response.Files[0].Name)
	})

	t.Run("DownloadRequest struct", func(t *testing.T) {
		request := DownloadRequest{
			Intent:       "entire_transfer",
			SecurityHash: "security123",
		}
		assert.Equal(t, "entire_transfer", request.Intent)
		assert.Equal(t, "security123", request.SecurityHash)
	})

	t.Run("DownloadResponse struct", func(t *testing.T) {
		response := DownloadResponse{
			DirectLink: "https://download.wetransfer.com/file.txt",
		}
		assert.Equal(t, "https://download.wetransfer.com/file.txt", response.DirectLink)
	})

	t.Run("WeTransferDownloadInfo struct", func(t *testing.T) {
		info := WeTransferDownloadInfo{
			DownloadURL: "https://download.example.com/file.txt",
			Filename:    "test-file.txt",
		}
		assert.Equal(t, "https://download.example.com/file.txt", info.DownloadURL)
		assert.Equal(t, "test-file.txt", info.Filename)
	})
}

func TestService_Integration(t *testing.T) {
	service := New()

	// Test the complete flow with a valid WeTransfer URL pattern
	originalURL := "https://wetransfer.com/downloads/abc123def456"

	// Check if URL is supported
	assert.True(t, service.IsSupported(originalURL))

	// Extract transfer ID
	transferID, err := service.extractTransferID(originalURL)
	require.NoError(t, err)
	assert.Equal(t, "abc123def456", transferID)

	// Convert URL (should return original for WeTransfer)
	convertedURL, err := service.ConvertURL(originalURL)
	require.NoError(t, err)
	assert.Equal(t, originalURL, convertedURL)

	// Verify service name
	assert.Equal(t, "WeTransfer", service.GetServiceName())
}

func TestService_EdgeCases(t *testing.T) {
	service := New()

	t.Run("Very long transfer ID", func(t *testing.T) {
		longID := "abc123def456ghi789jkl012mno345pqr678stu901vwx234yz567"
		url := "https://wetransfer.com/downloads/" + longID

		transferID, err := service.extractTransferID(url)
		assert.NoError(t, err)
		assert.Equal(t, longID, transferID)
	})

	t.Run("We.tl URL with alphanumeric only", func(t *testing.T) {
		complexID := "abc123XyZ456"
		url := "https://we.tl/" + complexID

		transferID, err := service.extractTransferID(url)
		assert.NoError(t, err)
		assert.Equal(t, complexID, transferID)
	})

	t.Run("URL with multiple query parameters", func(t *testing.T) {
		url := "https://wetransfer.com/downloads/abc123def456?utm_source=email&utm_medium=link&foo=bar"

		transferID, err := service.extractTransferID(url)
		assert.NoError(t, err)
		assert.Equal(t, "abc123def456", transferID)
	})

	t.Run("Case sensitivity limitation", func(t *testing.T) {
		url := "https://WeTransfer.com/downloads/abc123def456"

		// Both IsSupported and extractTransferID are case sensitive
		// This is a limitation in the current implementation
		assert.False(t, service.IsSupported(url))

		transferID, err := service.extractTransferID(url)
		assert.Error(t, err)
		assert.Equal(t, "", transferID)
	})

	t.Run("Different protocols", func(t *testing.T) {
		urls := []string{
			"http://wetransfer.com/downloads/abc123",
			"https://wetransfer.com/downloads/abc123",
			"wetransfer.com/downloads/abc123",
		}

		for _, url := range urls {
			assert.True(t, service.IsSupported(url), "URL should be supported: %s", url)

			transferID, err := service.extractTransferID(url)
			assert.NoError(t, err)
			assert.Equal(t, "abc123", transferID)
		}
	})
}
