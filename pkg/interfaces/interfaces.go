package interfaces

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// FileInfo contains metadata about a downloadable file
type FileInfo struct {
	URL           string
	Filename      string
	Size          int64
	SupportsRange bool
	ContentType   string
	LastModified  time.Time
}

// DownloadRequest represents a download request with all necessary parameters
type DownloadRequest struct {
	URL              string
	OutputPath       string
	CustomFilename   string
	MaxConnections   int
	ChunkSize        int64
	Timeout          time.Duration
	Resume           bool
	VerifyHash       string
	ProgressCallback func(downloaded, total int64)
}

// DownloadResult contains the results of a download operation
type DownloadResult struct {
	FilePath   string
	Size       int64
	Duration   time.Duration
	Speed      float64 // MB/s
	Hash       string
	Resumed    bool
	ChunksUsed int
}

// CloudService interface defines the contract for cloud service providers
type CloudService interface {
	// IsSupported checks if the service can handle the given URL
	IsSupported(url string) bool

	// GetServiceName returns the name of the service
	GetServiceName() string

	// ConvertURL converts a share URL to a direct download URL
	ConvertURL(url string) (string, error)

	// GetFileInfo retrieves metadata about the file
	GetFileInfo(ctx context.Context, url string) (*FileInfo, error)

	// PrepareDownload performs any necessary setup before downloading
	PrepareDownload(ctx context.Context, url string) (string, error)
}

// Downloader interface defines the main download functionality
type Downloader interface {
	// Download performs the actual file download
	Download(ctx context.Context, req *DownloadRequest) (*DownloadResult, error)

	// Resume resumes a partially downloaded file
	Resume(ctx context.Context, req *DownloadRequest) (*DownloadResult, error)

	// Cancel cancels an ongoing download
	Cancel() error

	// GetProgress returns current download progress
	GetProgress() (downloaded, total int64)
}

// ProgressTracker interface for tracking download progress
type ProgressTracker interface {
	// Start initializes the progress tracker
	Start(total int64, filename string)

	// Update updates the progress
	Update(downloaded int64)

	// Finish completes the progress tracking
	Finish()

	// SetError sets an error state
	SetError(err error)
}

// ChunkDownloader interface for downloading individual chunks
type ChunkDownloader interface {
	// DownloadChunk downloads a specific byte range
	DownloadChunk(ctx context.Context, url string, start, end int64) ([]byte, error)
}

// HashVerifier interface for file integrity verification
type HashVerifier interface {
	// CalculateHash calculates the hash of a file
	CalculateHash(filePath string, algorithm string) (string, error)

	// VerifyHash verifies a file against an expected hash
	VerifyHash(filePath string, expectedHash string, algorithm string) error
}

// ResumeManager interface for handling download resumption
type ResumeManager interface {
	// SaveProgress saves download progress for resumption
	SaveProgress(url string, progress *ResumeData) error

	// LoadProgress loads saved download progress
	LoadProgress(url string) (*ResumeData, error)

	// ClearProgress removes saved progress data
	ClearProgress(url string) error
}

// ResumeData contains information needed to resume a download
type ResumeData struct {
	URL          string    `json:"url"`
	FilePath     string    `json:"file_path"`
	TotalSize    int64     `json:"total_size"`
	Downloaded   int64     `json:"downloaded"`
	ChunkSize    int64     `json:"chunk_size"`
	LastModified time.Time `json:"last_modified"`
	Hash         string    `json:"hash,omitempty"`
}

// HTTPClient interface for making HTTP requests
type HTTPClient interface {
	// Get performs a GET request
	Get(url string) (*http.Response, error)

	// Head performs a HEAD request
	Head(url string) (*http.Response, error)

	// GetWithRange performs a GET request with Range header
	GetWithRange(url string, start, end int64) (*http.Response, error)

	// SetTimeout sets request timeout
	SetTimeout(timeout time.Duration)

	// SetRetry configures retry behavior
	SetRetry(attempts int, delay time.Duration)
}

// Error types for better error handling
type DownloadError struct {
	Type    string
	Message string
	URL     string
	Err     error
}

func (e *DownloadError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s (URL: %s): %v", e.Type, e.Message, e.URL, e.Err)
	}
	return fmt.Sprintf("%s: %s (URL: %s)", e.Type, e.Message, e.URL)
}

func (e *DownloadError) Unwrap() error {
	return e.Err
}

// Common error types
var (
	ErrUnsupportedURL    = &DownloadError{Type: "UnsupportedURL", Message: "URL not supported by any service"}
	ErrFileNotFound      = &DownloadError{Type: "FileNotFound", Message: "File not found"}
	ErrNetworkError      = &DownloadError{Type: "NetworkError", Message: "Network error occurred"}
	ErrInvalidResponse   = &DownloadError{Type: "InvalidResponse", Message: "Invalid response from server"}
	ErrHashMismatch      = &DownloadError{Type: "HashMismatch", Message: "File hash verification failed"}
	ErrInsufficientSpace = &DownloadError{Type: "InsufficientSpace", Message: "Insufficient disk space"}
	ErrPermissionDenied  = &DownloadError{Type: "PermissionDenied", Message: "Permission denied"}
)
