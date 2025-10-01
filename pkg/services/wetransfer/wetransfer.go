package wetransfer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

type WeTransferFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type WeTransferResponse struct {
	Files        []WeTransferFile `json:"files"`
	SecurityHash string           `json:"security_hash"`
}

type DownloadRequest struct {
	Intent       string `json:"intent"`
	SecurityHash string `json:"security_hash"`
}

type DownloadResponse struct {
	DirectLink string `json:"direct_link"`
}

func New() *Service {
	return &Service{
		httpClient: utils.NewHTTPClient(),
		logger:     logrus.New(),
	}
}

func (s *Service) IsSupported(rawURL string) bool {
	return strings.Contains(rawURL, "wetransfer.com") ||
		strings.Contains(rawURL, "we.tl")
}

func (s *Service) GetServiceName() string {
	return "WeTransfer"
}

func (s *Service) ConvertURL(rawURL string) (string, error) {
	if !s.IsSupported(rawURL) {
		return "", fmt.Errorf("not a valid WeTransfer URL: %s", rawURL)
	}

	// WeTransfer URLs need API interaction to get download links
	// Return the original URL and handle the conversion in PrepareDownload
	return rawURL, nil
}

func (s *Service) extractTransferID(rawURL string) (string, error) {
	// Pattern for we.tl short URLs
	re1 := regexp.MustCompile(`we\.tl/([a-zA-Z0-9]+)`)
	if matches := re1.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	// Pattern for full wetransfer.com URLs
	re2 := regexp.MustCompile(`wetransfer\.com/downloads/([a-zA-Z0-9]+)`)
	if matches := re2.FindStringSubmatch(rawURL); len(matches) > 1 {
		return matches[1], nil
	}

	return "", fmt.Errorf("no transfer ID found in URL")
}

func (s *Service) GetFileInfo(ctx context.Context, rawURL string) (*interfaces.FileInfo, error) {
	downloadInfo, err := s.getWeTransferDownloadInfo(ctx, rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get WeTransfer download info: %w", err)
	}

	s.logger.Infof("Getting file info for WeTransfer URL: %s", downloadInfo.DownloadURL)

	// Use HTTP client to get file info from the actual download URL
	httpFileInfo, err := s.httpClient.GetFileInfo(ctx, downloadInfo.DownloadURL, s.getDefaultHeaders())
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Convert utils.FileInfo to interfaces.FileInfo
	fileInfo := &interfaces.FileInfo{
		URL:           httpFileInfo.URL,
		Filename:      downloadInfo.Filename,
		Size:          httpFileInfo.Size,
		SupportsRange: httpFileInfo.SupportsRangeRequests,
		ContentType:   "", // Not available in utils.FileInfo
	}

	if httpFileInfo.LastModified != nil {
		fileInfo.LastModified = *httpFileInfo.LastModified
	}

	// Use the filename from WeTransfer API if available
	if fileInfo.Filename == "" {
		fileInfo.Filename = "wetransfer_file"
	}

	return fileInfo, nil
}

func (s *Service) PrepareDownload(ctx context.Context, rawURL string) (string, error) {
	downloadInfo, err := s.getWeTransferDownloadInfo(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("failed to get WeTransfer download info: %w", err)
	}

	return downloadInfo.DownloadURL, nil
}

type WeTransferDownloadInfo struct {
	DownloadURL string
	Filename    string
}

func (s *Service) getWeTransferDownloadInfo(ctx context.Context, rawURL string) (*WeTransferDownloadInfo, error) {
	transferID, err := s.extractTransferID(rawURL)
	if err != nil {
		return nil, err
	}

	s.logger.Infof("Extracted transfer ID: %s", transferID)

	// First, get the transfer information
	transferURL := fmt.Sprintf("https://wetransfer.com/api/v4/transfers/%s", transferID)

	req, err := http.NewRequestWithContext(ctx, "GET", transferURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	headers := s.getDefaultHeaders()
	headers["Accept"] = "application/json"
	headers["X-Requested-With"] = "XMLHttpRequest"

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get transfer info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var transferData WeTransferResponse
	if err := json.Unmarshal(body, &transferData); err != nil {
		return nil, fmt.Errorf("failed to parse transfer data: %w", err)
	}

	if len(transferData.Files) == 0 {
		return nil, fmt.Errorf("no files found in transfer")
	}

	// Get the first file's information
	firstFile := transferData.Files[0]

	// Request download URL
	downloadURL := fmt.Sprintf("https://wetransfer.com/api/v4/transfers/%s/download", transferID)

	downloadPayload := DownloadRequest{
		Intent:       "entire_transfer",
		SecurityHash: transferData.SecurityHash,
	}

	payloadBytes, err := json.Marshal(downloadPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal download payload: %w", err)
	}

	downloadReq, err := http.NewRequestWithContext(ctx, "POST", downloadURL, strings.NewReader(string(payloadBytes)))
	if err != nil {
		return nil, fmt.Errorf("failed to create download request: %w", err)
	}

	// Add headers for POST request
	headers["Content-Type"] = "application/json"
	for key, value := range headers {
		downloadReq.Header.Set(key, value)
	}

	downloadResp, err := client.Do(downloadReq)
	if err != nil {
		return nil, fmt.Errorf("failed to request download URL: %w", err)
	}
	defer downloadResp.Body.Close()

	if downloadResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected download request status code: %d", downloadResp.StatusCode)
	}

	downloadBody, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read download response body: %w", err)
	}

	var downloadData DownloadResponse
	if err := json.Unmarshal(downloadBody, &downloadData); err != nil {
		return nil, fmt.Errorf("failed to parse download response: %w", err)
	}

	if downloadData.DirectLink == "" {
		return nil, fmt.Errorf("no direct download link received")
	}

	return &WeTransferDownloadInfo{
		DownloadURL: downloadData.DirectLink,
		Filename:    firstFile.Name,
	}, nil
}

func (s *Service) getDefaultHeaders() map[string]string {
	return map[string]string{
		"Accept-Encoding": "identity",
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	}
}
