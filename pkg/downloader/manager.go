package downloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/milindmadhukar/cloudget/pkg/interfaces"
	"github.com/milindmadhukar/cloudget/pkg/services/dropbox"
	"github.com/milindmadhukar/cloudget/pkg/services/gdrive"
	"github.com/milindmadhukar/cloudget/pkg/services/wetransfer"
	"github.com/milindmadhukar/cloudget/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	services   []interfaces.CloudService
	httpClient *utils.HTTPClient
	logger     *logrus.Logger
	options    *ManagerOptions
}

type ManagerOptions struct {
	MaxConnections int
	ChunkSize      int64
	Timeout        time.Duration
	OutputDir      string
	Resume         bool
	VerifyHash     bool
	HashAlgorithm  string
}

func NewManager(options *ManagerOptions) *Manager {
	if options == nil {
		options = &ManagerOptions{
			MaxConnections: 8,
			ChunkSize:      2 * 1024 * 1024, // 2MB
			Timeout:        300 * time.Second,
			OutputDir:      ".",
			Resume:         true,
			VerifyHash:     false,
			HashAlgorithm:  "sha256",
		}
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	manager := &Manager{
		services:   make([]interfaces.CloudService, 0),
		httpClient: utils.NewHTTPClient(),
		logger:     logger,
		options:    options,
	}

	manager.httpClient.SetLogger(logger)

	// Register all available services
	manager.RegisterAllServices()

	return manager
}

func (m *Manager) RegisterAllServices() {
	// Register Dropbox service
	dropboxService := dropbox.New(m.logger)
	m.RegisterService(dropboxService)

	// Register Google Drive service
	gdriveService := gdrive.New()
	m.RegisterService(gdriveService)

	// Register WeTransfer service
	wetransferService := wetransfer.New()
	m.RegisterService(wetransferService)

	m.logger.Infof("Registered %d services", len(m.services))
}

func (m *Manager) RegisterService(service interfaces.CloudService) {
	m.services = append(m.services, service)
	m.logger.Debugf("Registered service: %s", service.GetServiceName())
}

func (m *Manager) SetLogger(logger *logrus.Logger) {
	m.logger = logger
	m.httpClient.SetLogger(logger)
}

func (m *Manager) FindService(url string) interfaces.CloudService {
	for _, service := range m.services {
		if service.IsSupported(url) {
			return service
		}
	}
	return nil
}

func (m *Manager) Download(ctx context.Context, req *interfaces.DownloadRequest) (*interfaces.DownloadResult, error) {
	startTime := time.Now()

	// Find appropriate service for the URL
	service := m.FindService(req.URL)
	if service == nil {
		return nil, fmt.Errorf("no service found for URL: %s", req.URL)
	}

	m.logger.Infof("Using service: %s", service.GetServiceName())

	// Get file information
	fileInfo, err := service.GetFileInfo(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Prepare download URL
	downloadURL, err := service.PrepareDownload(ctx, req.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare download: %w", err)
	}

	// Determine output path
	outputPath, err := m.determineOutputPath(req, fileInfo.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to determine output path: %w", err)
	}

	// Check if file already exists and is complete
	if m.options.Resume {
		if existingSize, exists := m.checkExistingFile(outputPath, fileInfo.Size); exists {
			m.logger.Infof("File already exists and is complete: %s", outputPath)

			duration := time.Since(startTime)
			return &interfaces.DownloadResult{
				FilePath:   outputPath,
				Size:       existingSize,
				Duration:   duration,
				Speed:      0, // No actual download occurred
				Resumed:    false,
				ChunksUsed: 0,
			}, nil
		}
	}

	m.logger.Infof("Starting download: %s -> %s", fileInfo.Filename, outputPath)

	// Prepare download options
	downloadOptions := &utils.DownloadOptions{
		ChunkSize:  m.options.ChunkSize,
		MaxRetries: 3,
		RetryDelay: 2 * time.Second,
		Headers:    make(map[string]string),
		UserAgent:  "Go-Cloud-Downloader/1.0",
		Timeout:    m.options.Timeout,
		ProgressFunc: func(downloaded, total int64) {
			percentage := float64(downloaded) / float64(total) * 100
			m.logger.Debugf("Progress: %.1f%% (%s / %s)",
				percentage,
				utils.FormatBytes(downloaded),
				utils.FormatBytes(total))
		},
	}

	// Perform the download
	err = m.httpClient.DownloadToFile(ctx, downloadURL, outputPath, downloadOptions)
	if err != nil {
		// Clean up partial file on error
		if _, statErr := os.Stat(outputPath); statErr == nil {
			os.Remove(outputPath)
		}
		return nil, fmt.Errorf("download failed: %w", err)
	}

	// Verify file size
	finalFileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat downloaded file: %w", err)
	}

	if finalFileInfo.Size() != fileInfo.Size {
		return nil, fmt.Errorf("file size mismatch: expected %d, got %d", fileInfo.Size, finalFileInfo.Size())
	}

	// Hash verification if requested
	var hash string
	if m.options.VerifyHash && req.VerifyHash != "" {
		m.logger.Info("Verifying file hash...")
		hashCalculator := utils.NewHashCalculator()
		calculatedHash, err := hashCalculator.CalculateHash(outputPath, m.options.HashAlgorithm)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate hash: %w", err)
		}

		if !strings.EqualFold(calculatedHash, req.VerifyHash) {
			return nil, fmt.Errorf("hash verification failed: expected %s, got %s", req.VerifyHash, calculatedHash)
		}

		hash = calculatedHash
		m.logger.Info("Hash verification passed")
	}

	duration := time.Since(startTime)
	speed := float64(fileInfo.Size) / duration.Seconds() / 1024 / 1024 // MB/s

	m.logger.Infof("Download completed successfully!")
	m.logger.Infof("File: %s", outputPath)
	m.logger.Infof("Size: %s", utils.FormatBytes(fileInfo.Size))
	m.logger.Infof("Time: %.1f seconds", duration.Seconds())
	m.logger.Infof("Speed: %.1f MB/s", speed)

	return &interfaces.DownloadResult{
		FilePath:   outputPath,
		Size:       fileInfo.Size,
		Duration:   duration,
		Speed:      speed,
		Hash:       hash,
		Resumed:    false, // TODO: Implement resume detection
		ChunksUsed: 0,     // TODO: Track chunks used
	}, nil
}

func (m *Manager) determineOutputPath(req *interfaces.DownloadRequest, detectedFilename string) (string, error) {
	var outputPath string

	if req.OutputPath != "" {
		// Use explicit output path
		outputPath = req.OutputPath
	} else {
		// Determine filename
		filename := req.CustomFilename
		if filename == "" {
			filename = detectedFilename
		}
		if filename == "" {
			filename = "download"
		}

		// Use output directory from request or manager options
		outputDir := m.options.OutputDir
		if req.OutputPath != "" {
			outputDir = filepath.Dir(req.OutputPath)
		}

		outputPath = filepath.Join(outputDir, filename)
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	return outputPath, nil
}

func (m *Manager) checkExistingFile(outputPath string, expectedSize int64) (int64, bool) {
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, false
		}
		m.logger.Warnf("Error checking existing file: %v", err)
		return 0, false
	}

	actualSize := fileInfo.Size()
	if actualSize == expectedSize {
		return actualSize, true
	}

	m.logger.Infof("Existing file size mismatch (expected: %d, actual: %d), will re-download", expectedSize, actualSize)
	return actualSize, false
}

func (m *Manager) Resume(ctx context.Context, req *interfaces.DownloadRequest) (*interfaces.DownloadResult, error) {
	// TODO: Implement proper resume functionality using utils/resume.go
	m.logger.Warn("Resume functionality not yet implemented, performing full download")
	return m.Download(ctx, req)
}

func (m *Manager) Cancel() error {
	// TODO: Implement cancellation
	m.logger.Warn("Cancel functionality not yet implemented")
	return nil
}

func (m *Manager) GetProgress() (downloaded, total int64) {
	// TODO: Implement progress retrieval from progress manager
	return 0, 0
}
