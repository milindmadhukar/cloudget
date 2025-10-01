package dropbox

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/milindmadhukar/cloudget/pkg/interfaces"
	"github.com/sirupsen/logrus"
)

type Service struct {
	logger *logrus.Logger
}

func New(logger *logrus.Logger) *Service {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	return &Service{
		logger: logger,
	}
}

func (s *Service) IsSupported(urlStr string) bool {
	return strings.Contains(urlStr, "dropbox.com")
}

func (s *Service) GetServiceName() string {
	return "Dropbox"
}

func (s *Service) ConvertURL(urlStr string) (string, error) {
	if !s.IsSupported(urlStr) {
		return "", fmt.Errorf("not a valid Dropbox URL")
	}

	// Handle different Dropbox URL formats
	if strings.Contains(urlStr, "/s/") || strings.Contains(urlStr, "/scl/fi/") {
		if strings.Contains(urlStr, "dl=0") {
			return strings.Replace(urlStr, "dl=0", "dl=1", 1), nil
		} else if strings.Contains(urlStr, "?") {
			return urlStr + "&dl=1", nil
		} else {
			return urlStr + "?dl=1", nil
		}
	}

	return "", fmt.Errorf("unsupported Dropbox URL format")
}

func (s *Service) GetFileInfo(ctx context.Context, urlStr string) (*interfaces.FileInfo, error) {
	// This would typically make an HTTP HEAD request to get file metadata
	// For now, we'll return basic info - this should be implemented with actual HTTP calls

	downloadURL, err := s.ConvertURL(urlStr)
	if err != nil {
		return nil, err
	}

	filename := s.extractFilename(urlStr)
	if filename == "" {
		filename = "downloaded_file"
	}

	return &interfaces.FileInfo{
		URL:           downloadURL,
		Filename:      filename,
		Size:          0,    // Would be determined from HEAD request
		SupportsRange: true, // Dropbox typically supports range requests
		ContentType:   "application/octet-stream",
	}, nil
}

func (s *Service) PrepareDownload(ctx context.Context, urlStr string) (string, error) {
	return s.ConvertURL(urlStr)
}

func (s *Service) extractFilename(urlStr string) string {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}

	// Decode URL path
	decodedPath, err := url.QueryUnescape(parsedURL.Path)
	if err != nil {
		decodedPath = parsedURL.Path
	}

	// Handle Dropbox URL structure
	if strings.Contains(decodedPath, "/s/") {
		// Format: /s/hash/filename
		parts := strings.Split(decodedPath, "/")
		if len(parts) >= 4 {
			filename := parts[len(parts)-1]
			if filename != "" {
				return filename
			}
		}
	} else if strings.Contains(decodedPath, "/scl/fi/") {
		// New format: extract filename from path
		parts := strings.Split(decodedPath, "/")
		for i := len(parts) - 1; i >= 0; i-- {
			part := parts[i]
			if part != "" && strings.Contains(part, ".") {
				return part
			}
		}
	}

	return ""
}

// ValidateURL validates Dropbox URL format more strictly
func (s *Service) ValidateURL(urlStr string) error {
	if !s.IsSupported(urlStr) {
		return interfaces.ErrUnsupportedURL
	}

	parsed, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	if parsed.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	// Check for known Dropbox URL patterns
	validPatterns := []string{
		`dropbox\.com/s/[a-zA-Z0-9]+/.*`,
		`dropbox\.com/scl/fi/[a-zA-Z0-9]+/.*`,
	}

	urlString := strings.ToLower(urlStr)
	for _, pattern := range validPatterns {
		matched, err := regexp.MatchString(pattern, urlString)
		if err == nil && matched {
			return nil
		}
	}

	return fmt.Errorf("unsupported Dropbox URL format")
}
