package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/milindmadhukar/cloudget/pkg/interfaces"
)

// ResumeManager handles saving and loading download progress for resumption
type ResumeManager struct {
	resumeDir string
}

// NewResumeManager creates a new resume manager
func NewResumeManager(resumeDir string) *ResumeManager {
	if resumeDir == "" {
		resumeDir = filepath.Join(os.TempDir(), "cloudget-resume")
	}

	// Ensure resume directory exists
	os.MkdirAll(resumeDir, 0755)

	return &ResumeManager{
		resumeDir: resumeDir,
	}
}

// SaveProgress saves download progress for resumption
func (rm *ResumeManager) SaveProgress(url string, progress *interfaces.ResumeData) error {
	filename := rm.getResumeFilename(url)
	filepath := filepath.Join(rm.resumeDir, filename)

	data, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resume data: %w", err)
	}

	err = os.WriteFile(filepath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write resume file: %w", err)
	}

	return nil
}

// LoadProgress loads saved download progress
func (rm *ResumeManager) LoadProgress(url string) (*interfaces.ResumeData, error) {
	filename := rm.getResumeFilename(url)
	filepath := filepath.Join(rm.resumeDir, filename)

	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No resume data found
		}
		return nil, fmt.Errorf("failed to read resume file: %w", err)
	}

	var progress interfaces.ResumeData
	err = json.Unmarshal(data, &progress)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal resume data: %w", err)
	}

	return &progress, nil
}

// ClearProgress removes saved progress data
func (rm *ResumeManager) ClearProgress(url string) error {
	filename := rm.getResumeFilename(url)
	filepath := filepath.Join(rm.resumeDir, filename)

	err := os.Remove(filepath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove resume file: %w", err)
	}

	return nil
}

// IsResumable checks if a download can be resumed
func (rm *ResumeManager) IsResumable(url string, outputPath string) (bool, *interfaces.ResumeData, error) {
	progress, err := rm.LoadProgress(url)
	if err != nil {
		return false, nil, err
	}

	if progress == nil {
		return false, nil, nil
	}

	// Check if the output file exists and matches the saved progress
	if progress.FilePath != outputPath {
		return false, nil, nil
	}

	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil, nil
		}
		return false, nil, fmt.Errorf("failed to stat output file: %w", err)
	}

	// Verify file size matches saved progress
	if fileInfo.Size() != progress.Downloaded {
		return false, nil, nil
	}

	// Check if file was modified after the resume data was saved
	if fileInfo.ModTime().After(progress.LastModified) {
		return false, nil, nil
	}

	return true, progress, nil
}

// CleanupOldResumeData removes resume data older than the specified duration
func (rm *ResumeManager) CleanupOldResumeData(ctx context.Context, maxAge time.Duration) error {
	entries, err := os.ReadDir(rm.resumeDir)
	if err != nil {
		return fmt.Errorf("failed to read resume directory: %w", err)
	}

	cutoff := time.Now().Add(-maxAge)

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			filepath := filepath.Join(rm.resumeDir, entry.Name())
			os.Remove(filepath) // Ignore errors for cleanup
		}
	}

	return nil
}

// getResumeFilename generates a safe filename for resume data based on URL
func (rm *ResumeManager) getResumeFilename(url string) string {
	// Create a simple hash-like filename based on URL
	// In a real implementation, you'd want to properly hash the URL
	filename := ""
	for i, r := range url {
		if i >= 20 {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			filename += string(r)
		} else {
			filename += "_"
		}
	}
	return fmt.Sprintf("resume_%s.json", filename)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
