package dropbox

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		logger *logrus.Logger
	}{
		{
			name:   "with logger",
			logger: logrus.New(),
		},
		{
			name:   "with nil logger",
			logger: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := New(tt.logger)
			if service == nil {
				t.Fatal("New() returned nil")
			}
			if service.logger == nil {
				t.Fatal("Service logger is nil")
			}
		})
	}
}

func TestService_GetServiceName(t *testing.T) {
	service := New(nil)
	name := service.GetServiceName()
	if name != "Dropbox" {
		t.Errorf("GetServiceName() = %s, want Dropbox", name)
	}
}

func TestService_IsSupported(t *testing.T) {
	service := New(nil)

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{
			name: "valid dropbox URL",
			url:  "https://www.dropbox.com/s/abc123/file.pdf?dl=0",
			want: true,
		},
		{
			name: "valid dropbox URL without www",
			url:  "https://dropbox.com/s/abc123/file.pdf?dl=0",
			want: true,
		},
		{
			name: "dropbox scl URL",
			url:  "https://dropbox.com/scl/fi/abc123/file.pdf?dl=0",
			want: true,
		},
		{
			name: "non-dropbox URL",
			url:  "https://google.com/file.pdf",
			want: false,
		},
		{
			name: "empty URL",
			url:  "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.IsSupported(tt.url)
			if result != tt.want {
				t.Errorf("IsSupported(%s) = %v, want %v", tt.url, result, tt.want)
			}
		})
	}
}

func TestService_ConvertURL(t *testing.T) {
	service := New(nil)

	tests := []struct {
		name        string
		url         string
		expected    string
		expectError bool
	}{
		{
			name:     "dropbox URL with dl=0",
			url:      "https://dropbox.com/s/abc123/file.pdf?dl=0",
			expected: "https://dropbox.com/s/abc123/file.pdf?dl=1",
		},
		{
			name:     "dropbox URL without query params",
			url:      "https://dropbox.com/s/abc123/file.pdf",
			expected: "https://dropbox.com/s/abc123/file.pdf?dl=1",
		},
		{
			name:     "dropbox URL with other query params",
			url:      "https://dropbox.com/s/abc123/file.pdf?foo=bar",
			expected: "https://dropbox.com/s/abc123/file.pdf?foo=bar&dl=1",
		},
		{
			name:     "dropbox scl URL",
			url:      "https://dropbox.com/scl/fi/abc123/file.pdf?dl=0",
			expected: "https://dropbox.com/scl/fi/abc123/file.pdf?dl=1",
		},
		{
			name:        "non-dropbox URL",
			url:         "https://google.com/file.pdf",
			expectError: true,
		},
		{
			name:        "invalid dropbox format",
			url:         "https://dropbox.com/invalid/format",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.ConvertURL(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("ConvertURL(%s) = %s, want %s", tt.url, result, tt.expected)
			}
		})
	}
}

func TestService_extractFilename(t *testing.T) {
	service := New(nil)

	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "dropbox s URL with filename",
			url:      "https://dropbox.com/s/abc123/document.pdf",
			expected: "document.pdf",
		},
		{
			name:     "dropbox s URL with encoded filename",
			url:      "https://dropbox.com/s/abc123/file%20with%20spaces.pdf",
			expected: "file with spaces.pdf",
		},
		{
			name:     "dropbox scl URL with filename",
			url:      "https://dropbox.com/scl/fi/abc123xyz/report.docx?dl=0",
			expected: "report.docx",
		},
		{
			name:     "dropbox scl URL with path containing filename",
			url:      "https://dropbox.com/scl/fi/abc123/somefile.txt/more/path",
			expected: "somefile.txt",
		},
		{
			name:     "URL without clear filename",
			url:      "https://dropbox.com/other/format",
			expected: "",
		},
		{
			name:     "invalid URL",
			url:      "not-a-url",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.extractFilename(tt.url)
			if result != tt.expected {
				t.Errorf("extractFilename(%s) = %s, want %s", tt.url, result, tt.expected)
			}
		})
	}
}

func TestService_GetFileInfo(t *testing.T) {
	service := New(nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name: "valid dropbox URL",
			url:  "https://dropbox.com/s/abc123/file.pdf?dl=0",
		},
		{
			name:        "invalid URL",
			url:         "https://google.com/file.pdf",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileInfo, err := service.GetFileInfo(ctx, tt.url)

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
				t.Fatal("FileInfo is nil")
			}

			if fileInfo.URL == "" {
				t.Error("FileInfo.URL is empty")
			}

			if fileInfo.Filename == "" {
				t.Error("FileInfo.Filename is empty")
			}

			if !fileInfo.SupportsRange {
				t.Error("Expected SupportsRange to be true for Dropbox")
			}
		})
	}
}

func TestService_PrepareDownload(t *testing.T) {
	service := New(nil)
	ctx := context.Background()

	url := "https://dropbox.com/s/abc123/file.pdf?dl=0"
	expected := "https://dropbox.com/s/abc123/file.pdf?dl=1"

	result, err := service.PrepareDownload(ctx, url)
	if err != nil {
		t.Fatalf("PrepareDownload failed: %v", err)
	}

	if result != expected {
		t.Errorf("PrepareDownload() = %s, want %s", result, expected)
	}
}

func TestService_ValidateURL(t *testing.T) {
	service := New(nil)

	tests := []struct {
		name        string
		url         string
		expectError bool
	}{
		{
			name: "valid dropbox s URL",
			url:  "https://dropbox.com/s/abc123/file.pdf",
		},
		{
			name: "valid dropbox scl URL",
			url:  "https://dropbox.com/scl/fi/abc123/file.pdf",
		},
		{
			name: "valid dropbox URL with numbers",
			url:  "https://dropbox.com/s/abc123def456/document.pdf?dl=0",
		},
		{
			name:        "non-dropbox URL",
			url:         "https://google.com/file.pdf",
			expectError: true,
		},
		{
			name:        "invalid dropbox format",
			url:         "https://dropbox.com/invalid/format",
			expectError: true,
		},
		{
			name:        "dropbox URL without path",
			url:         "https://dropbox.com",
			expectError: true,
		},
		{
			name:        "malformed URL",
			url:         "not-a-url",
			expectError: true,
		},
		{
			name:        "URL without host",
			url:         "/path/only",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateURL(tt.url)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
