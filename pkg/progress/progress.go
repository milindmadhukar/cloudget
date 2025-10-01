package progress

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
)

type Tracker struct {
	mu           sync.RWMutex
	downloads    map[string]*DownloadProgress
	logger       *logrus.Logger
	showProgress bool
}

type DownloadProgress struct {
	mu          sync.RWMutex
	ID          string
	Filename    string
	TotalBytes  int64
	Downloaded  int64
	StartTime   time.Time
	LastUpdate  time.Time
	Speed       float64 // bytes per second
	ETA         time.Duration
	Status      DownloadStatus
	Error       error
	ProgressBar *progressbar.ProgressBar
	chunks      map[int]*ChunkProgress
	chunksMu    sync.RWMutex
}

type ChunkProgress struct {
	ID         int
	Start      int64
	End        int64
	Downloaded int64
	Status     ChunkStatus
}

type DownloadStatus int

const (
	StatusPending DownloadStatus = iota
	StatusRunning
	StatusCompleted
	StatusFailed
	StatusPaused
	StatusCancelled
)

type ChunkStatus int

const (
	ChunkPending ChunkStatus = iota
	ChunkDownloading
	ChunkCompleted
	ChunkFailed
)

func (s DownloadStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusRunning:
		return "Running"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	case StatusPaused:
		return "Paused"
	case StatusCancelled:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

func NewTracker(logger *logrus.Logger, showProgress bool) *Tracker {
	if logger == nil {
		logger = logrus.New()
		logger.SetLevel(logrus.InfoLevel)
	}

	return &Tracker{
		downloads:    make(map[string]*DownloadProgress),
		logger:       logger,
		showProgress: showProgress,
	}
}

func (t *Tracker) StartDownload(id, filename string, totalBytes int64) *DownloadProgress {
	t.mu.Lock()
	defer t.mu.Unlock()

	var progressBar *progressbar.ProgressBar
	if t.showProgress {
		progressBar = progressbar.NewOptions64(
			totalBytes,
			progressbar.OptionSetDescription(filename),
			progressbar.OptionSetWriter(io.Discard), // We'll handle output ourselves
			progressbar.OptionShowBytes(true),
			progressbar.OptionSetWidth(50),
			progressbar.OptionThrottle(65*time.Millisecond),
			progressbar.OptionShowCount(),
			progressbar.OptionSpinnerType(14),
			progressbar.OptionFullWidth(),
		)
	}

	progress := &DownloadProgress{
		ID:          id,
		Filename:    filename,
		TotalBytes:  totalBytes,
		Downloaded:  0,
		StartTime:   time.Now(),
		LastUpdate:  time.Now(),
		Status:      StatusRunning,
		ProgressBar: progressBar,
		chunks:      make(map[int]*ChunkProgress),
	}

	t.downloads[id] = progress

	if t.showProgress {
		t.logger.Infof("Started downloading: %s (%s)", filename, formatBytes(totalBytes))
	}

	return progress
}

func (t *Tracker) UpdateProgress(id string, downloaded int64) {
	t.mu.RLock()
	progress, exists := t.downloads[id]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.mu.Lock()
	defer progress.mu.Unlock()

	now := time.Now()
	timeDiff := now.Sub(progress.LastUpdate).Seconds()

	if timeDiff > 0 {
		bytesDiff := downloaded - progress.Downloaded
		progress.Speed = float64(bytesDiff) / timeDiff
	}

	progress.Downloaded = downloaded
	progress.LastUpdate = now

	if progress.TotalBytes > 0 && progress.Speed > 0 {
		remaining := progress.TotalBytes - progress.Downloaded
		progress.ETA = time.Duration(float64(remaining)/progress.Speed) * time.Second
	}

	if progress.ProgressBar != nil {
		progress.ProgressBar.Set64(downloaded)
	}
}

func (t *Tracker) UpdateChunkProgress(downloadID string, chunkID int, downloaded int64) {
	t.mu.RLock()
	progress, exists := t.downloads[downloadID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.chunksMu.Lock()
	chunk, exists := progress.chunks[chunkID]
	if exists {
		chunk.Downloaded = downloaded
		if chunk.Downloaded >= (chunk.End - chunk.Start + 1) {
			chunk.Status = ChunkCompleted
		}
	}
	progress.chunksMu.Unlock()

	// Calculate total downloaded across all chunks
	var totalDownloaded int64
	progress.chunksMu.RLock()
	for _, c := range progress.chunks {
		totalDownloaded += c.Downloaded
	}
	progress.chunksMu.RUnlock()

	t.UpdateProgress(downloadID, totalDownloaded)
}

func (t *Tracker) AddChunk(downloadID string, chunkID int, start, end int64) {
	t.mu.RLock()
	progress, exists := t.downloads[downloadID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.chunksMu.Lock()
	progress.chunks[chunkID] = &ChunkProgress{
		ID:         chunkID,
		Start:      start,
		End:        end,
		Downloaded: 0,
		Status:     ChunkPending,
	}
	progress.chunksMu.Unlock()
}

func (t *Tracker) SetChunkStatus(downloadID string, chunkID int, status ChunkStatus) {
	t.mu.RLock()
	progress, exists := t.downloads[downloadID]
	t.mu.RUnlock()

	if !exists {
		return
	}

	progress.chunksMu.Lock()
	if chunk, exists := progress.chunks[chunkID]; exists {
		chunk.Status = status
	}
	progress.chunksMu.Unlock()
}

func (t *Tracker) CompleteDownload(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	progress, exists := t.downloads[id]
	if !exists {
		return
	}

	progress.Status = StatusCompleted
	progress.Downloaded = progress.TotalBytes

	if progress.ProgressBar != nil {
		progress.ProgressBar.Finish()
	}

	duration := time.Since(progress.StartTime)
	avgSpeed := float64(progress.TotalBytes) / duration.Seconds()

	t.logger.Infof("Completed: %s (%s in %v, avg speed: %s/s)",
		progress.Filename,
		formatBytes(progress.TotalBytes),
		duration.Round(time.Second),
		formatBytes(int64(avgSpeed)))
}

func (t *Tracker) FailDownload(id string, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	progress, exists := t.downloads[id]
	if !exists {
		return
	}

	progress.Status = StatusFailed
	progress.Error = err

	if progress.ProgressBar != nil {
		progress.ProgressBar.Finish()
	}

	t.logger.Errorf("Failed: %s - %v", progress.Filename, err)
}

func (t *Tracker) GetProgress(id string) (*DownloadProgress, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	progress, exists := t.downloads[id]
	return progress, exists
}

func (t *Tracker) GetAllProgress() map[string]*DownloadProgress {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*DownloadProgress)
	for id, progress := range t.downloads {
		result[id] = progress
	}
	return result
}

func (t *Tracker) RemoveDownload(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if progress, exists := t.downloads[id]; exists {
		if progress.ProgressBar != nil {
			progress.ProgressBar.Finish()
		}
		delete(t.downloads, id)
	}
}

func (t *Tracker) PrintSummary() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if len(t.downloads) == 0 {
		return
	}

	fmt.Println("\n=== Download Summary ===")

	var completed, failed, running int
	var totalBytes, downloadedBytes int64

	for _, progress := range t.downloads {
		switch progress.Status {
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		case StatusRunning:
			running++
		}

		totalBytes += progress.TotalBytes
		downloadedBytes += progress.Downloaded
	}

	fmt.Printf("Total Downloads: %d\n", len(t.downloads))
	fmt.Printf("Completed: %d, Failed: %d, Running: %d\n", completed, failed, running)
	fmt.Printf("Total Size: %s, Downloaded: %s\n",
		formatBytes(totalBytes), formatBytes(downloadedBytes))

	if failed > 0 {
		fmt.Println("\nFailed Downloads:")
		for _, progress := range t.downloads {
			if progress.Status == StatusFailed {
				fmt.Printf("  - %s: %v\n", progress.Filename, progress.Error)
			}
		}
	}
}

// ProgressWriter wraps an io.Writer and reports write progress
type ProgressWriter struct {
	writer     io.Writer
	tracker    *Tracker
	downloadID string
	written    int64
}

func NewProgressWriter(writer io.Writer, tracker *Tracker, downloadID string) *ProgressWriter {
	return &ProgressWriter{
		writer:     writer,
		tracker:    tracker,
		downloadID: downloadID,
	}
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n, err := pw.writer.Write(p)
	if n > 0 {
		pw.written += int64(n)
		pw.tracker.UpdateProgress(pw.downloadID, pw.written)
	}
	return n, err
}

// ProgressReader wraps an io.Reader and reports read progress
type ProgressReader struct {
	reader     io.Reader
	tracker    *Tracker
	downloadID string
	read       int64
}

func NewProgressReader(reader io.Reader, tracker *Tracker, downloadID string) *ProgressReader {
	return &ProgressReader{
		reader:     reader,
		tracker:    tracker,
		downloadID: downloadID,
	}
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.read += int64(n)
		pr.tracker.UpdateProgress(pr.downloadID, pr.read)
	}
	return n, err
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// CreateProgressCallback creates a callback function for HTTP client progress reporting
func (t *Tracker) CreateProgressCallback(downloadID string) func(downloaded, total int64) {
	return func(downloaded, total int64) {
		t.UpdateProgress(downloadID, downloaded)
	}
}

// WaitForCompletion waits for all downloads to complete or fail
func (t *Tracker) WaitForCompletion(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			t.mu.RLock()
			allDone := true
			for _, progress := range t.downloads {
				if progress.Status == StatusRunning || progress.Status == StatusPending {
					allDone = false
					break
				}
			}
			t.mu.RUnlock()

			if allDone {
				return nil
			}
		}
	}
}
