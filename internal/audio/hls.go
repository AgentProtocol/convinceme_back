package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	maxSegments   = 5                   // Maximum number of segments to keep
	segmentLength = 4                   // Length of each segment in seconds
	playlistFile  = "playlist.m3u8"     // Master playlist filename
	segmentFormat = "segment_%d_%s.aac" // Format for segment filenames
)

type HLSManager struct {
	baseDir     string
	mu          sync.RWMutex
	segments    map[string][]string // Map of agent -> segment files
	lastSegment map[string]int      // Map of agent -> last segment number
}

func NewHLSManager(baseDir string) (*HLSManager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create HLS directory: %v", err)
	}

	return &HLSManager{
		baseDir:     baseDir,
		segments:    make(map[string][]string),
		lastSegment: make(map[string]int),
	}, nil
}

// AddSegment adds a new segment to the HLS stream
func (h *HLSManager) AddSegment(data []byte, agentName string) (string, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Initialize agent's segments if not exists
	if _, exists := h.segments[agentName]; !exists {
		h.segments[agentName] = make([]string, 0)
		h.lastSegment[agentName] = 0
	}

	// Generate new segment number
	h.lastSegment[agentName]++
	segmentNum := h.lastSegment[agentName]

	// Create segment filename
	agentNameWithDashes := strings.Replace(agentName, " ", "_", -1)
	segmentName := fmt.Sprintf(segmentFormat, segmentNum, agentNameWithDashes)
	segmentPath := filepath.Join(h.baseDir, segmentName)

	// Write segment file
	if err := os.WriteFile(segmentPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write segment file: %v", err)
	}

	// Add to segments list
	h.segments[agentName] = append(h.segments[agentName], segmentName)

	// Keep only last N segments
	if len(h.segments[agentName]) > maxSegments {
		oldestSegment := h.segments[agentName][0]
		h.segments[agentName] = h.segments[agentName][1:]

		// Remove old segment file
		oldPath := filepath.Join(h.baseDir, oldestSegment)
		os.Remove(oldPath)
	}

	// Update playlist with full URLs
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
	content += fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", segmentLength)
	content += "#EXT-X-MEDIA-SEQUENCE:0\n"

	// Add segments for each agent with full URLs
	for agent, segments := range h.segments {
		content += fmt.Sprintf("\n# Agent: %s\n", agent)
		for _, segment := range segments {
			content += fmt.Sprintf("#EXTINF:%.3f,\n", float64(segmentLength))
			content += fmt.Sprintf("http://localhost:8080/hls/%s\n", segment)
		}
	}

	// Write playlist file
	return os.WriteFile(playlistPath, []byte(content), 0644)
}

// GetPlaylistPath returns the path to the playlist file
func (h *HLSManager) GetPlaylistPath() string {
	return filepath.Join(h.baseDir, playlistFile)
}

// CleanupOldSegments removes segments older than maxAge
func (h *HLSManager) CleanupOldSegments() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for agent := range h.segments {
		if len(h.segments[agent]) > maxSegments {
			// Remove old segments
			oldSegments := h.segments[agent][:len(h.segments[agent])-maxSegments]
			h.segments[agent] = h.segments[agent][len(h.segments[agent])-maxSegments:]

			// Delete old segment files
			for _, segment := range oldSegments {
				os.Remove(filepath.Join(h.baseDir, segment))
			}
		}
	}
}
