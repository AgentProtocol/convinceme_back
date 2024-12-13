package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	maxSegments   = 4                   // Maximum number of segments to keep
	segmentLength = 4                   // Length of each segment in seconds
	playlistFile  = "playlist.m3u8"     // Master playlist filename
	segmentFormat = "segment_%d_%s.aac" // Format for segment filenames
)

type HLSManager struct {
	baseDir     string
	mu          sync.RWMutex
	segments    []string // Single list of segments in chronological order
	lastSegment int      // Global segment counter
	mediaSeq    int      // Global media sequence number
}

func NewHLSManager(baseDir string) (*HLSManager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create HLS directory: %v", err)
	}

	return &HLSManager{
		baseDir:     baseDir,
		segments:    make([]string, 0),
		lastSegment: 0,
		mediaSeq:    0,
	}, nil
}

// AddSegment adds a new segment to the HLS stream
func (h *HLSManager) AddSegment(data []byte, agentName string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Generate new segment number
	h.lastSegment++

	// Create segment filename
	agentNameWithDashes := strings.Replace(strings.TrimSpace(agentName), " ", "_", -1)
	segmentName := fmt.Sprintf(segmentFormat, h.lastSegment, agentNameWithDashes)
	segmentPath := filepath.Join(h.baseDir, segmentName)

	// Write segment file
	if err := os.WriteFile(segmentPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write segment file: %v", err)
	}

	// Add to segments list
	h.segments = append(h.segments, segmentName)

	// Keep only last N segments
	if len(h.segments) > maxSegments {
		// Remove old segments
		numToRemove := len(h.segments) - maxSegments
		oldSegments := h.segments[:numToRemove]
		h.segments = h.segments[numToRemove:]

		// Increment media sequence by number of removed segments
		h.mediaSeq += numToRemove

		// Delete old segment files
		for _, oldSegment := range oldSegments {
			os.Remove(filepath.Join(h.baseDir, oldSegment))
		}
	}

	// Update playlist
	if err := h.updatePlaylist(); err != nil {
		return "", fmt.Errorf("failed to update playlist: %v", err)
	}

	return segmentName, nil
}

// updatePlaylist updates the HLS playlist file
func (h *HLSManager) updatePlaylist() error {
	playlistPath := filepath.Join(h.baseDir, playlistFile)

	// Create playlist content
	content := "#EXTM3U\n"
	content += "#EXT-X-VERSION:3\n"
	content += "#EXT-X-ALLOW-CACHE:NO\n"
	content += fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", segmentLength)
	content += fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d\n", h.mediaSeq)

	// Add all segments in chronological order
	fmt.Printf("Current segments in playlist: %v\n", h.segments)
	for _, segment := range h.segments {
		content += fmt.Sprintf("#EXTINF:%.3f,\n", float64(segmentLength))
		content += fmt.Sprintf("http://localhost:8080/hls/%s\n", segment)
	}

	fmt.Printf("Writing playlist content:\n%s\n", content)

	// Write playlist file atomically
	tempFile := playlistPath + ".tmp"
	if err := os.WriteFile(tempFile, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write temp playlist: %v", err)
	}
	return os.Rename(tempFile, playlistPath)
}

// GetPlaylistPath returns the path to the playlist file
func (h *HLSManager) GetPlaylistPath() string {
	return filepath.Join(h.baseDir, playlistFile)
}
