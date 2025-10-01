package main

import (
	"context"
	"testing"
	"time"

	"github.com/cloud-downloader/downloader/pkg/downloader"
	"github.com/cloud-downloader/downloader/pkg/interfaces"
)

func TestManagerInitialization(t *testing.T) {
	manager := downloader.NewManager(nil)
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}

	// Test that services are registered
	testURLs := map[string]string{
		"dropbox":    "https://dropbox.com/s/abc123/test.txt",
		"gdrive":     "https://drive.google.com/file/d/abc123/view",
		"wetransfer": "https://we.tl/t-abc123",
	}

	for serviceName, testURL := range testURLs {
		service := manager.FindService(testURL)
		if service == nil {
			t.Errorf("No service found for %s URL: %s", serviceName, testURL)
		} else {
			t.Logf("Found service '%s' for %s", service.GetServiceName(), serviceName)
		}
	}
}

func TestManagerOptions(t *testing.T) {
	options := &downloader.ManagerOptions{
		MaxConnections: 16,
		ChunkSize:      4 * 1024 * 1024,
		Timeout:        600 * time.Second,
		OutputDir:      "/tmp/test",
		Resume:         false,
		VerifyHash:     true,
		HashAlgorithm:  "sha512",
	}

	manager := downloader.NewManager(options)
	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
}

func TestDownloadRequestCreation(t *testing.T) {
	req := &interfaces.DownloadRequest{
		URL:              "https://dropbox.com/s/test123/example.txt",
		OutputPath:       "/tmp/test.txt",
		CustomFilename:   "custom.txt",
		MaxConnections:   4,
		ChunkSize:        1024 * 1024,
		Timeout:          300 * time.Second,
		Resume:           true,
		VerifyHash:       "abc123",
		ProgressCallback: nil,
	}

	if req.URL == "" {
		t.Error("URL should not be empty")
	}

	if req.ChunkSize <= 0 {
		t.Error("ChunkSize should be positive")
	}
}

func TestServiceRegistration(t *testing.T) {
	manager := downloader.NewManager(nil)

	// Test that we can find services for different URL patterns
	testCases := []struct {
		url         string
		shouldFind  bool
		serviceName string
	}{
		{"https://dropbox.com/s/abc123/file.txt", true, "Dropbox"},
		{"https://dropbox.com/scl/fi/abc123/file.txt", true, "Dropbox"},
		{"https://drive.google.com/file/d/abc123/view", true, "Google Drive"},
		{"https://docs.google.com/document/d/abc123/edit", true, "Google Drive"},
		{"https://we.tl/t-abc123", true, "WeTransfer"},
		{"https://wetransfer.com/downloads/abc123", true, "WeTransfer"},
		{"https://example.com/file.txt", false, ""},
		{"https://unsupported.com/file.txt", false, ""},
	}

	for _, tc := range testCases {
		service := manager.FindService(tc.url)
		if tc.shouldFind {
			if service == nil {
				t.Errorf("Expected to find service for URL %s", tc.url)
			} else if service.GetServiceName() != tc.serviceName {
				t.Errorf("Expected service %s for URL %s, got %s", tc.serviceName, tc.url, service.GetServiceName())
			}
		} else {
			if service != nil {
				t.Errorf("Did not expect to find service for URL %s, but found %s", tc.url, service.GetServiceName())
			}
		}
	}
}

func TestDownloadManagerInterface(t *testing.T) {
	manager := downloader.NewManager(nil)

	// Test that manager implements expected interface methods
	ctx := context.Background()

	// Test with invalid URL (should fail gracefully)
	req := &interfaces.DownloadRequest{
		URL: "https://unsupported.example.com/file.txt",
	}

	_, err := manager.Download(ctx, req)
	if err == nil {
		t.Error("Expected error for unsupported URL")
	}

	// Test GetProgress (should not panic)
	downloaded, total := manager.GetProgress()
	t.Logf("Progress: %d/%d", downloaded, total)

	// Test Cancel (should not panic)
	err = manager.Cancel()
	if err != nil {
		t.Logf("Cancel returned error (expected): %v", err)
	}
}
