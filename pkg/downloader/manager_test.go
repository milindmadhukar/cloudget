package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/milindmadhukar/cloudget/pkg/interfaces"
)

type mockService struct {
	name              string
	supportedFn       func(string) bool
	getInfoFn         func(context.Context, string) (*interfaces.FileInfo, error)
	convertURLFn      func(string) (string, error)
	prepareDownloadFn func(context.Context, string) (string, error)
}

func (m *mockService) GetServiceName() string {
	return m.name
}

func (m *mockService) IsSupported(url string) bool {
	if m.supportedFn != nil {
		return m.supportedFn(url)
	}
	return strings.Contains(url, m.name)
}

func (m *mockService) ConvertURL(url string) (string, error) {
	if m.convertURLFn != nil {
		return m.convertURLFn(url)
	}
	return url, nil
}

func (m *mockService) GetFileInfo(ctx context.Context, url string) (*interfaces.FileInfo, error) {
	if m.getInfoFn != nil {
		return m.getInfoFn(ctx, url)
	}
	return &interfaces.FileInfo{
		Filename:    "test-file.txt",
		Size:        1024,
		URL:         url,
		ContentType: "text/plain",
	}, nil
}

func (m *mockService) PrepareDownload(ctx context.Context, url string) (string, error) {
	if m.prepareDownloadFn != nil {
		return m.prepareDownloadFn(ctx, url)
	}
	return url, nil
}

func TestNewManager(t *testing.T) {
	tests := []struct {
		name     string
		options  *ManagerOptions
		expected *ManagerOptions
	}{
		{
			name:    "default options",
			options: nil,
			expected: &ManagerOptions{
				MaxConnections: 8,
				ChunkSize:      2 * 1024 * 1024,
				Timeout:        300 * time.Second,
				OutputDir:      ".",
				Resume:         true,
				VerifyHash:     false,
				HashAlgorithm:  "sha256",
			},
		},
		{
			name: "custom options",
			options: &ManagerOptions{
				MaxConnections: 4,
				ChunkSize:      1024 * 1024,
				Timeout:        60 * time.Second,
				OutputDir:      "/tmp",
				Resume:         false,
				VerifyHash:     true,
				HashAlgorithm:  "md5",
			},
			expected: &ManagerOptions{
				MaxConnections: 4,
				ChunkSize:      1024 * 1024,
				Timeout:        60 * time.Second,
				OutputDir:      "/tmp",
				Resume:         false,
				VerifyHash:     true,
				HashAlgorithm:  "md5",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(tt.options)

			if manager.options.MaxConnections != tt.expected.MaxConnections {
				t.Errorf("MaxConnections = %d, want %d", manager.options.MaxConnections, tt.expected.MaxConnections)
			}
			if manager.options.ChunkSize != tt.expected.ChunkSize {
				t.Errorf("ChunkSize = %d, want %d", manager.options.ChunkSize, tt.expected.ChunkSize)
			}
			if manager.options.Timeout != tt.expected.Timeout {
				t.Errorf("Timeout = %v, want %v", manager.options.Timeout, tt.expected.Timeout)
			}
			if manager.options.OutputDir != tt.expected.OutputDir {
				t.Errorf("OutputDir = %q, want %q", manager.options.OutputDir, tt.expected.OutputDir)
			}
			if manager.options.Resume != tt.expected.Resume {
				t.Errorf("Resume = %v, want %v", manager.options.Resume, tt.expected.Resume)
			}
			if manager.options.VerifyHash != tt.expected.VerifyHash {
				t.Errorf("VerifyHash = %v, want %v", manager.options.VerifyHash, tt.expected.VerifyHash)
			}
			if manager.options.HashAlgorithm != tt.expected.HashAlgorithm {
				t.Errorf("HashAlgorithm = %q, want %q", manager.options.HashAlgorithm, tt.expected.HashAlgorithm)
			}

			if len(manager.services) == 0 {
				t.Error("Expected services to be registered by default, got none")
			}
		})
	}
}

func TestManager_RegisterService(t *testing.T) {
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      ".",
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	initialCount := len(manager.services)

	service1 := &mockService{name: "service1"}
	service2 := &mockService{name: "service2"}

	manager.RegisterService(service1)
	if len(manager.services) != initialCount+1 {
		t.Fatalf("Expected %d services, got %d", initialCount+1, len(manager.services))
	}
	if manager.services[initialCount].GetServiceName() != "service1" {
		t.Errorf("Expected service1, got %s", manager.services[initialCount].GetServiceName())
	}

	manager.RegisterService(service2)
	if len(manager.services) != initialCount+2 {
		t.Fatalf("Expected %d services, got %d", initialCount+2, len(manager.services))
	}
}

func TestManager_RegisterAllServices(t *testing.T) {
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      ".",
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	if len(manager.services) == 0 {
		t.Fatal("Expected services to be registered, got none")
	}

	serviceNames := make(map[string]bool)
	for _, service := range manager.services {
		serviceNames[service.GetServiceName()] = true
	}

	expectedServices := []string{"Dropbox", "Google Drive", "WeTransfer"}
	for _, expected := range expectedServices {
		if !serviceNames[expected] {
			t.Errorf("Expected service %s to be registered", expected)
		}
	}
}

func TestManager_FindService(t *testing.T) {
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      ".",
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service1 := &mockService{
		name: "service1",
		supportedFn: func(url string) bool {
			return strings.Contains(url, "service1.com")
		},
	}
	service2 := &mockService{
		name: "service2",
		supportedFn: func(url string) bool {
			return strings.Contains(url, "service2.com")
		},
	}

	manager.RegisterService(service1)
	manager.RegisterService(service2)

	tests := []struct {
		name     string
		url      string
		expected string
		isNil    bool
	}{
		{
			name:     "finds service1",
			url:      "https://service1.com/file/123",
			expected: "service1",
			isNil:    false,
		},
		{
			name:     "finds service2",
			url:      "https://service2.com/download/456",
			expected: "service2",
			isNil:    false,
		},
		{
			name:  "no matching service",
			url:   "https://unknown.com/file/789",
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := manager.FindService(tt.url)

			if tt.isNil {
				if service != nil {
					t.Error("Expected nil service")
				}
			} else {
				if service == nil {
					t.Fatal("Expected service, got nil")
				}
				if service.GetServiceName() != tt.expected {
					t.Errorf("Expected service %s, got %s", tt.expected, service.GetServiceName())
				}
			}
		})
	}
}

func TestManager_determineOutputPath(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name             string
		outputDir        string
		req              *interfaces.DownloadRequest
		detectedFilename string
		expected         string
		shouldFail       bool
	}{
		{
			name:      "explicit output path",
			outputDir: tmpDir,
			req: &interfaces.DownloadRequest{
				OutputPath: filepath.Join(tmpDir, "explicit.txt"),
			},
			detectedFilename: "original.txt",
			expected:         filepath.Join(tmpDir, "explicit.txt"),
		},
		{
			name:      "custom filename",
			outputDir: tmpDir,
			req: &interfaces.DownloadRequest{
				CustomFilename: "custom.txt",
			},
			detectedFilename: "original.txt",
			expected:         filepath.Join(tmpDir, "custom.txt"),
		},
		{
			name:             "use detected filename",
			outputDir:        tmpDir,
			req:              &interfaces.DownloadRequest{},
			detectedFilename: "detected.txt",
			expected:         filepath.Join(tmpDir, "detected.txt"),
		},
		{
			name:             "fallback to default",
			outputDir:        tmpDir,
			req:              &interfaces.DownloadRequest{},
			detectedFilename: "",
			expected:         filepath.Join(tmpDir, "download"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := &Manager{
				options: &ManagerOptions{OutputDir: tt.outputDir},
			}

			result, err := manager.determineOutputPath(tt.req, tt.detectedFilename)

			if tt.shouldFail {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("Expected path %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestManager_checkExistingFile(t *testing.T) {
	tmpDir := t.TempDir()

	testFile := filepath.Join(tmpDir, "existing.txt")
	content := "existing content"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name           string
		filePath       string
		fileSize       int64
		expectedSize   int64
		expectedExists bool
	}{
		{
			name:           "file doesn't exist",
			filePath:       filepath.Join(tmpDir, "nonexistent.txt"),
			fileSize:       100,
			expectedSize:   0,
			expectedExists: false,
		},
		{
			name:           "file exists, same size",
			filePath:       testFile,
			fileSize:       int64(len(content)),
			expectedSize:   int64(len(content)),
			expectedExists: true,
		},
		{
			name:           "file exists, different size",
			filePath:       testFile,
			fileSize:       100,
			expectedSize:   int64(len(content)),
			expectedExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewManager(&ManagerOptions{
				MaxConnections: 8,
				ChunkSize:      2 * 1024 * 1024,
				Timeout:        300 * time.Second,
				OutputDir:      tmpDir,
				Resume:         true,
				VerifyHash:     false,
				HashAlgorithm:  "sha256",
			})

			size, exists := manager.checkExistingFile(tt.filePath, tt.fileSize)

			if size != tt.expectedSize {
				t.Errorf("Expected size=%d, got %d", tt.expectedSize, size)
			}
			if exists != tt.expectedExists {
				t.Errorf("Expected exists=%v, got %v", tt.expectedExists, exists)
			}
		})
	}
}

func TestManager_Download_Success(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "test file content"
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, content)
	}))
	defer server.Close()

	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return strings.Contains(url, "test.com")
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			return &interfaces.FileInfo{
				Filename:    "test-file.txt",
				Size:        17, // len("test file content")
				URL:         url,
				ContentType: "text/plain",
			}, nil
		},
		prepareDownloadFn: func(ctx context.Context, url string) (string, error) {
			return server.URL, nil
		},
	}

	manager.RegisterService(service)

	req := &interfaces.DownloadRequest{
		URL:            "https://test.com/file/123",
		CustomFilename: "downloaded.txt",
	}

	result, err := manager.Download(context.Background(), req)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	expectedPath := filepath.Join(tmpDir, "downloaded.txt")
	if result.FilePath != expectedPath {
		t.Errorf("Expected path %q, got %q", expectedPath, result.FilePath)
	}

	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}
}

func TestManager_Download_ServiceNotFound(t *testing.T) {
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      t.TempDir(),
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	// Clear all services to simulate no matching service
	manager.services = make([]interfaces.CloudService, 0)

	req := &interfaces.DownloadRequest{
		URL: "https://unsupported.com/file/123",
	}

	_, err := manager.Download(context.Background(), req)
	if err == nil {
		t.Error("Expected error for unsupported URL, got nil")
	}

	if !strings.Contains(err.Error(), "no service found") {
		t.Errorf("Expected 'no service found' error, got: %v", err)
	}
}

func TestManager_Download_GetFileInfoError(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return true
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			return nil, fmt.Errorf("failed to get file info")
		},
	}

	manager.RegisterService(service)

	req := &interfaces.DownloadRequest{
		URL: "https://test.com/file/123",
	}

	_, err := manager.Download(context.Background(), req)
	if err == nil {
		t.Error("Expected error from GetFileInfo, got nil")
	}

	if !strings.Contains(err.Error(), "failed to get file info") {
		t.Errorf("Expected file info error, got: %v", err)
	}
}

func TestManager_Download_PrepareDownloadError(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return true
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			return &interfaces.FileInfo{
				Filename: "test.txt",
				Size:     1024,
				URL:      url,
			}, nil
		},
		prepareDownloadFn: func(ctx context.Context, url string) (string, error) {
			return "", fmt.Errorf("prepare download failed")
		},
	}

	manager.RegisterService(service)

	req := &interfaces.DownloadRequest{
		URL: "https://test.com/file/123",
	}

	_, err := manager.Download(context.Background(), req)
	if err == nil {
		t.Error("Expected prepare download error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to prepare download") {
		t.Errorf("Expected prepare download error, got: %v", err)
	}
}

func TestManager_Download_FileAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()

	existingFile := filepath.Join(tmpDir, "existing.txt")
	content := "existing content"
	err := os.WriteFile(existingFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return true
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			return &interfaces.FileInfo{
				Filename: "existing.txt",
				Size:     int64(len(content)),
				URL:      url,
			}, nil
		},
	}

	manager.RegisterService(service)

	req := &interfaces.DownloadRequest{
		URL:            "https://test.com/file/123",
		CustomFilename: "existing.txt",
	}

	result, err := manager.Download(context.Background(), req)
	if err != nil {
		t.Errorf("Download should skip existing file, got error: %v", err)
	}

	if result.Speed != 0 {
		t.Error("Expected speed to be 0 for skipped download")
	}
}

func TestManager_Resume(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "test content for resume"
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, content)
	}))
	defer server.Close()

	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return true
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			return &interfaces.FileInfo{
				Filename: "resumed.txt",
				Size:     23, // len("test content for resume")
				URL:      url,
			}, nil
		},
		prepareDownloadFn: func(ctx context.Context, url string) (string, error) {
			return server.URL, nil
		},
	}

	manager.RegisterService(service)

	req := &interfaces.DownloadRequest{
		URL:            "https://test.com/file/123",
		CustomFilename: "resumed.txt",
	}

	result, err := manager.Resume(context.Background(), req)
	if err != nil {
		t.Errorf("Resume failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	expectedPath := filepath.Join(tmpDir, "resumed.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Resumed file does not exist")
	}
}

func TestManager_Download_WithContext(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     false,
		HashAlgorithm:  "sha256",
	})

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return true
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				return &interfaces.FileInfo{
					Filename: "test.txt",
					Size:     1024,
					URL:      url,
				}, nil
			}
		},
	}

	manager.RegisterService(service)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	req := &interfaces.DownloadRequest{
		URL: "https://test.com/file/123",
	}

	_, err := manager.Download(ctx, req)
	if err == nil {
		t.Error("Expected context cancellation error, got nil")
	}

	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected context canceled error, got: %v", err)
	}
}

func TestManager_Download_WithHashVerification(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "test"
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Content-Type", "text/plain")
		io.WriteString(w, content)
	}))
	defer server.Close()

	manager := NewManager(&ManagerOptions{
		MaxConnections: 8,
		ChunkSize:      2 * 1024 * 1024,
		Timeout:        300 * time.Second,
		OutputDir:      tmpDir,
		Resume:         true,
		VerifyHash:     true,
		HashAlgorithm:  "sha256",
	})

	expectedHash := "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"

	service := &mockService{
		name: "test-service",
		supportedFn: func(url string) bool {
			return true
		},
		getInfoFn: func(ctx context.Context, url string) (*interfaces.FileInfo, error) {
			return &interfaces.FileInfo{
				Filename: "test.txt",
				Size:     4,
				URL:      url,
			}, nil
		},
		prepareDownloadFn: func(ctx context.Context, url string) (string, error) {
			return server.URL, nil
		},
	}

	manager.RegisterService(service)

	req := &interfaces.DownloadRequest{
		URL:        "https://test.com/file/123",
		VerifyHash: expectedHash,
	}

	result, err := manager.Download(context.Background(), req)
	if err != nil {
		t.Errorf("Download with hash verification failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.Hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, result.Hash)
	}
}

func TestManager_GetProgress(t *testing.T) {
	manager := NewManager(nil)

	downloaded, total := manager.GetProgress()
	if downloaded != 0 || total != 0 {
		t.Errorf("Expected (0, 0), got (%d, %d)", downloaded, total)
	}
}

func TestManager_Cancel(t *testing.T) {
	manager := NewManager(nil)

	err := manager.Cancel()
	if err != nil {
		t.Errorf("Cancel should not return error for unimplemented functionality, got: %v", err)
	}
}
