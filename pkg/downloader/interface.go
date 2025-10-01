package downloader

import (
	"github.com/cloud-downloader/downloader/pkg/interfaces"
)

// Type aliases for backward compatibility
type FileInfo = interfaces.FileInfo
type DownloadRequest = interfaces.DownloadRequest
type DownloadResult = interfaces.DownloadResult
type CloudService = interfaces.CloudService
type Downloader = interfaces.Downloader
type ProgressTracker = interfaces.ProgressTracker
type ChunkDownloader = interfaces.ChunkDownloader
type HashVerifier = interfaces.HashVerifier
type ResumeManager = interfaces.ResumeManager
type ResumeData = interfaces.ResumeData
type HTTPClient = interfaces.HTTPClient
type DownloadError = interfaces.DownloadError

// Re-export common error types
var (
	ErrUnsupportedURL    = interfaces.ErrUnsupportedURL
	ErrFileNotFound      = interfaces.ErrFileNotFound
	ErrNetworkError      = interfaces.ErrNetworkError
	ErrInvalidResponse   = interfaces.ErrInvalidResponse
	ErrHashMismatch      = interfaces.ErrHashMismatch
	ErrInsufficientSpace = interfaces.ErrInsufficientSpace
	ErrPermissionDenied  = interfaces.ErrPermissionDenied
)
