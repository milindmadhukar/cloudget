package gdrive

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/milindmadhukar/cloudget/pkg/interfaces"
	"github.com/milindmadhukar/cloudget/pkg/utils"
	"github.com/sirupsen/logrus"
)

type Service struct {
	httpClient *utils.HTTPClient
	logger     *logrus.Logger
}

func New() *Service {
	return &Service{
		httpClient: utils.NewHTTPClient(),
		logger:     logrus.New(),
	}
}

func (s *Service) IsSupported(rawURL string) bool {
	return strings.Contains(rawURL, "drive.google.com") ||
		strings.Contains(rawURL, "docs.google.com")
}

func (s *Service) GetServiceName() string {
	return "Google Drive"
}

func (s *Service) ConvertURL(rawURL string) (string, error) {
	if !s.IsSupported(rawURL) {
		return "", fmt.Errorf("not a valid Google Drive URL: %s", rawURL)
	}

	fileID, err := s.extractFileID(rawURL)
	if err != nil {
		return "", fmt.Errorf("could not extract file ID from Google Drive URL: %w", err)
	}

	// For large files, Google Drive requires additional parameters
	return fmt.Sprintf("https://drive.google.com/uc?export=download&id=%s&confirm=t", fileID), nil
}

func (s *Service) extractFileID(rawURL string) (string, error) {
	// Pattern 1: /file/d/{file_id}/
	re1 := regexp.MustCompile(`/file/d/([a-zA-Z0-9_-]+)`)
	if matches := re1.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	// Pattern 2: id={file_id}
	re2 := regexp.MustCompile(`[?&]id=([a-zA-Z0-9_-]+)`)
	if matches := re2.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	// Pattern 3: /open?id={file_id}
	re3 := regexp.MustCompile(`/open\?id=([a-zA-Z0-9_-]+)`)
	if matches := re3.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	// Pattern 4: /d/{file_id}
	re4 := regexp.MustCompile(`/d/([a-zA-Z0-9_-]+)`)
	if matches := re4.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("no file ID found in URL")
}

func (s *Service) GetFileInfo(ctx context.Context, rawURL string) (*interfaces.FileInfo, error) {
	downloadURL, err := s.ConvertURL(rawURL)
	if err != nil {
		return nil, err
	}

	s.logger.Infof("Getting file info for Google Drive URL: %s", downloadURL)

	// Check if we need to handle virus scan redirect
	finalURL, err := s.handleVirusScanRedirect(downloadURL)
	if err != nil {
		s.logger.Warnf("Could not handle virus scan redirect: %v", err)
		finalURL = downloadURL
	}

	// Use HTTP client to get file info
	httpFileInfo, err := s.httpClient.GetFileInfo(ctx, finalURL, s.getDefaultHeaders())
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Convert utils.FileInfo to interfaces.FileInfo
	fileInfo := &interfaces.FileInfo{
		URL:           httpFileInfo.URL,
		Filename:      httpFileInfo.Filename,
		Size:          httpFileInfo.Size,
		SupportsRange: httpFileInfo.SupportsRangeRequests,
		ContentType:   "", // Not available in utils.FileInfo
	}

	if httpFileInfo.LastModified != nil {
		fileInfo.LastModified = *httpFileInfo.LastModified
	}

	// Google Drive might not provide a filename in the headers initially
	// We'll try to extract it from Content-Disposition header or use a default
	if fileInfo.Filename == "" {
		fileInfo.Filename = "google_drive_file"
	}

	return fileInfo, nil
}

func (s *Service) PrepareDownload(ctx context.Context, rawURL string) (string, error) {
	downloadURL, err := s.ConvertURL(rawURL)
	if err != nil {
		return "", err
	}

	// Check if we need to handle virus scan redirect
	finalURL, err := s.handleVirusScanRedirect(downloadURL)
	if err != nil {
		s.logger.Warnf("Could not handle virus scan redirect: %v", err)
		finalURL = downloadURL
	}

	return finalURL, nil
}

func (s *Service) handleVirusScanRedirect(downloadURL string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Don't follow redirects automatically, we want to handle them
			return http.ErrUseLastResponse
		},
	}

	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}

	// Add default headers
	for key, value := range s.getDefaultHeaders() {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Check if we're being redirected to accounts.google.com or virus scan page
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		location := resp.Header.Get("Location")
		if strings.Contains(location, "accounts.google.com") ||
			strings.Contains(location, "drive.google.com/uc") {

			// Try to extract the actual download URL from the redirect
			parsedURL, err := url.Parse(location)
			if err != nil {
				return downloadURL, nil
			}

			// If there's a confirm parameter, use it
			if confirm := parsedURL.Query().Get("confirm"); confirm != "" {
				fileID, _ := s.extractFileID(downloadURL)
				return fmt.Sprintf("https://drive.google.com/uc?export=download&confirm=%s&id=%s", confirm, fileID), nil
			}
		}
	}

	return downloadURL, nil
}

func (s *Service) getDefaultHeaders() map[string]string {
	return map[string]string{
		"Accept-Encoding": "identity",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
}
