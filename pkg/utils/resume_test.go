package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/milindmadhukar/cloudget/pkg/interfaces"
)

func TestNewResumeManager(t *testing.T) {
	tests := []struct {
		name      string
		resumeDir string
		wantDir   string
	}{
		{
			name:      "with custom directory",
			resumeDir: "/tmp/custom-resume",
			wantDir:   "/tmp/custom-resume",
		},
		{
			name:      "with empty directory",
			resumeDir: "",
			wantDir:   filepath.Join(os.TempDir(), "cloudget-resume"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rm := NewResumeManager(tt.resumeDir)
			if rm == nil {
				t.Fatal("NewResumeManager() returned nil")
			}
			if rm.resumeDir != tt.wantDir {
				t.Errorf("resumeDir = %s, want %s", rm.resumeDir, tt.wantDir)
			}

			// Verify directory was created
			if _, err := os.Stat(rm.resumeDir); os.IsNotExist(err) {
				t.Errorf("Resume directory was not created: %s", rm.resumeDir)
			}

			// Cleanup
			os.RemoveAll(rm.resumeDir)
		})
	}
}

func TestResumeManager_SaveProgress(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	testURL := "https://example.com/file.zip"
	progressData := &interfaces.ResumeData{
		URL:          testURL,
		FilePath:     "/tmp/file.zip",
		TotalSize:    1000,
		Downloaded:   500,
		LastModified: time.Now(),
		Hash:         "abc123",
	}

	err := rm.SaveProgress(testURL, progressData)
	if err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	// Verify file was created
	filename := rm.getResumeFilename(testURL)
	resumePath := filepath.Join(tmpDir, filename)

	if _, err := os.Stat(resumePath); os.IsNotExist(err) {
		t.Errorf("Resume file was not created: %s", resumePath)
	}

	// Verify content
	data, err := os.ReadFile(resumePath)
	if err != nil {
		t.Fatalf("Failed to read resume file: %v", err)
	}

	var saved interfaces.ResumeData
	err = json.Unmarshal(data, &saved)
	if err != nil {
		t.Fatalf("Failed to unmarshal resume data: %v", err)
	}

	if saved.URL != progressData.URL {
		t.Errorf("URL = %s, want %s", saved.URL, progressData.URL)
	}
	if saved.Downloaded != progressData.Downloaded {
		t.Errorf("Downloaded = %d, want %d", saved.Downloaded, progressData.Downloaded)
	}
}

func TestResumeManager_LoadProgress(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	testURL := "https://example.com/file.zip"
	progressData := &interfaces.ResumeData{
		URL:          testURL,
		FilePath:     "/tmp/file.zip",
		TotalSize:    1000,
		Downloaded:   500,
		LastModified: time.Now(),
		Hash:         "abc123",
	}

	// Save first
	err := rm.SaveProgress(testURL, progressData)
	if err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	// Load and verify
	loaded, err := rm.LoadProgress(testURL)
	if err != nil {
		t.Fatalf("LoadProgress failed: %v", err)
	}

	if loaded == nil {
		t.Fatal("LoadProgress returned nil")
	}

	if loaded.URL != progressData.URL {
		t.Errorf("URL = %s, want %s", loaded.URL, progressData.URL)
	}
	if loaded.Downloaded != progressData.Downloaded {
		t.Errorf("Downloaded = %d, want %d", loaded.Downloaded, progressData.Downloaded)
	}
}

func TestResumeManager_LoadProgressNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	testURL := "https://example.com/nonexistent.zip"

	loaded, err := rm.LoadProgress(testURL)
	if err != nil {
		t.Fatalf("LoadProgress failed: %v", err)
	}

	if loaded != nil {
		t.Error("LoadProgress should return nil for non-existent file")
	}
}

func TestResumeManager_ClearProgress(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	testURL := "https://example.com/file.zip"
	progressData := &interfaces.ResumeData{
		URL:        testURL,
		FilePath:   "/tmp/file.zip",
		TotalSize:  1000,
		Downloaded: 500,
	}

	// Save first
	err := rm.SaveProgress(testURL, progressData)
	if err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	// Clear and verify
	err = rm.ClearProgress(testURL)
	if err != nil {
		t.Fatalf("ClearProgress failed: %v", err)
	}

	// Verify file was removed
	loaded, err := rm.LoadProgress(testURL)
	if err != nil {
		t.Fatalf("LoadProgress after clear failed: %v", err)
	}

	if loaded != nil {
		t.Error("Progress should be nil after clearing")
	}
}

func TestResumeManager_IsResumable(t *testing.T) {
	tmpDir := t.TempDir()

	testURL := "https://example.com/file.zip"
	outputPath := filepath.Join(tmpDir, "file.zip")

	// Create a test file
	testData := []byte("test file content")
	err := os.WriteFile(outputPath, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat test file: %v", err)
	}

	tests := []struct {
		name            string
		setupTest       func(*ResumeManager)
		url             string
		outputPath      string
		expectResumable bool
	}{
		{
			name: "resumable file",
			setupTest: func(rm *ResumeManager) {
				progressData := &interfaces.ResumeData{
					URL:          testURL,
					FilePath:     outputPath,
					TotalSize:    1000,
					Downloaded:   int64(len(testData)),
					LastModified: fileInfo.ModTime().Add(time.Hour),
				}
				rm.SaveProgress(testURL, progressData)
			},
			url:             testURL,
			outputPath:      outputPath,
			expectResumable: true,
		},
		{
			name: "different path",
			setupTest: func(rm *ResumeManager) {
				progressData := &interfaces.ResumeData{
					URL:          testURL,
					FilePath:     outputPath,
					TotalSize:    1000,
					Downloaded:   int64(len(testData)),
					LastModified: fileInfo.ModTime().Add(time.Hour),
				}
				rm.SaveProgress(testURL, progressData)
			},
			url:             testURL,
			outputPath:      "/different/path.zip",
			expectResumable: false,
		},
		{
			name: "non-existent URL",
			setupTest: func(rm *ResumeManager) {
				// Don't save any progress for this URL
			},
			url:             "https://example.com/other-unique-file.zip",
			outputPath:      outputPath,
			expectResumable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a separate resume manager for each test
			testTmpDir := t.TempDir()
			rm := NewResumeManager(testTmpDir)

			// Setup test specific data
			tt.setupTest(rm)

			resumable, data, err := rm.IsResumable(tt.url, tt.outputPath)
			if err != nil {
				t.Fatalf("IsResumable failed: %v", err)
			}

			if resumable != tt.expectResumable {
				t.Errorf("IsResumable = %v, want %v", resumable, tt.expectResumable)
			}

			if tt.expectResumable && data == nil {
				t.Error("Expected resume data for resumable file")
			}

			if !tt.expectResumable && data != nil {
				t.Error("Expected nil resume data for non-resumable file")
			}
		})
	}
}

func TestResumeManager_CleanupOldResumeData(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	// Create old resume file
	oldFile := filepath.Join(tmpDir, "old_resume.json")
	err := os.WriteFile(oldFile, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}

	// Make it old
	oldTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(oldFile, oldTime, oldTime)
	if err != nil {
		t.Fatalf("Failed to change file time: %v", err)
	}

	// Create new resume file
	newFile := filepath.Join(tmpDir, "new_resume.json")
	err = os.WriteFile(newFile, []byte("{}"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}

	ctx := context.Background()
	maxAge := time.Hour

	err = rm.CleanupOldResumeData(ctx, maxAge)
	if err != nil {
		t.Fatalf("CleanupOldResumeData failed: %v", err)
	}

	// Verify old file was removed
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("Old resume file should have been removed")
	}

	// Verify new file still exists
	if _, err := os.Stat(newFile); os.IsNotExist(err) {
		t.Error("New resume file should still exist")
	}
}

func TestResumeManager_getResumeFilename(t *testing.T) {
	rm := NewResumeManager("")

	tests := []struct {
		name string
		url  string
	}{
		{
			name: "basic URL",
			url:  "https://example.com/file.zip",
		},
		{
			name: "URL with special characters",
			url:  "https://example.com/file with spaces & symbols.zip",
		},
		{
			name: "very long URL",
			url:  "https://verylongdomainname.example.com/very/long/path/with/many/segments/file.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := rm.getResumeFilename(tt.url)

			if filename == "" {
				t.Error("getResumeFilename returned empty string")
			}

			if !strings.HasPrefix(filename, "resume_") {
				t.Errorf("Filename should start with 'resume_', got: %s", filename)
			}

			if !strings.HasSuffix(filename, ".json") {
				t.Errorf("Filename should end with '.json', got: %s", filename)
			}

			// Verify filename is safe for filesystem
			if strings.ContainsAny(filename, "/\\:*?\"<>|") {
				t.Errorf("Filename contains unsafe characters: %s", filename)
			}
		})
	}
}

func TestResumeManager_IsResumableFileSizeMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	testURL := "https://example.com/file.zip"
	outputPath := filepath.Join(tmpDir, "file.zip")

	// Create a test file
	testData := []byte("test file content")
	err := os.WriteFile(outputPath, testData, 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create progress data with different size
	progressData := &interfaces.ResumeData{
		URL:          testURL,
		FilePath:     outputPath,
		TotalSize:    1000,
		Downloaded:   999, // Different from actual file size
		LastModified: time.Now().Add(-time.Minute),
	}

	err = rm.SaveProgress(testURL, progressData)
	if err != nil {
		t.Fatalf("SaveProgress failed: %v", err)
	}

	resumable, _, err := rm.IsResumable(testURL, outputPath)
	if err != nil {
		t.Fatalf("IsResumable failed: %v", err)
	}

	if resumable {
		t.Error("File should not be resumable with size mismatch")
	}
}

func TestResumeManager_WithContext(t *testing.T) {
	tmpDir := t.TempDir()
	rm := NewResumeManager(tmpDir)

	// Create multiple old files
	for i := 0; i < 5; i++ {
		filename := filepath.Join(tmpDir, fmt.Sprintf("old_%d.json", i))
		err := os.WriteFile(filename, []byte("{}"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		oldTime := time.Now().Add(-2 * time.Hour)
		os.Chtimes(filename, oldTime, oldTime)
	}

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := rm.CleanupOldResumeData(ctx, time.Hour)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got: %v", err)
	}
}
