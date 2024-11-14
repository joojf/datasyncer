package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

type SyncState struct {
	ID             string                `json:"id"`
	StartTime      time.Time             `json:"start_time"`
	LastUpdated    time.Time             `json:"last_updated"`
	Status         string                `json:"status"`
	FileStates     map[string]FileState  `json:"file_states"`
	FailedFiles    map[string]FailedFile `json:"failed_files"`
	TotalFiles     int                   `json:"total_files"`
	ProcessedFiles int                   `json:"processed_files"`
}

type FileState struct {
	Path             string    `json:"path"`
	Size             int64     `json:"size"`
	LastModified     time.Time `json:"last_modified"`
	ETag             string    `json:"etag"`
	Status           string    `json:"status"` // "pending", "in_progress", "completed", "failed"
	BytesTransferred int64     `json:"bytes_transferred"`
	Attempts         int       `json:"attempts"`
}

type FailedFile struct {
	Path      string    `json:"path"`
	Error     string    `json:"error"`
	Timestamp time.Time `json:"timestamp"`
	Attempts  int       `json:"attempts"`
}

type RecoveryManager struct {
	statePath    string
	maxAttempts  int
	state        *SyncState
	mu           sync.RWMutex
	saveInterval time.Duration
}

func NewRecoveryManager(statePath string, maxAttempts int) (*RecoveryManager, error) {
	rm := &RecoveryManager{
		statePath:    statePath,
		maxAttempts:  maxAttempts,
		saveInterval: 30 * time.Second,
	}

	if err := rm.loadState(); err != nil {
		rm.state = &SyncState{
			ID:          fmt.Sprintf("sync_%d", time.Now().Unix()),
			StartTime:   time.Now(),
			Status:      "initializing",
			FileStates:  make(map[string]FileState),
			FailedFiles: make(map[string]FailedFile),
		}
	}

	return rm, nil
}

func (rm *RecoveryManager) loadState() error {
	data, err := os.ReadFile(rm.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read state file: %v", err)
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %v", err)
	}

	rm.state = &state
	return nil
}

func (rm *RecoveryManager) saveState() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.state.LastUpdated = time.Now()
	data, err := json.MarshalIndent(rm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %v", err)
	}

	tempFile := rm.statePath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %v", err)
	}

	return os.Rename(tempFile, rm.statePath)
}

func (rm *RecoveryManager) StartAutoSave(ctx context.Context) {
	ticker := time.NewTicker(rm.saveInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				rm.saveState() // Final save
				return
			case <-ticker.C:
				if err := rm.saveState(); err != nil {
					// Log error but continue
					fmt.Fprintf(os.Stderr, "Failed to save state: %v\n", err)
				}
			}
		}
	}()
}

func (rm *RecoveryManager) GetFileState(path string) (FileState, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	state, exists := rm.state.FileStates[path]
	return state, exists
}

func (rm *RecoveryManager) UpdateFileState(state FileState) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.state.FileStates[state.Path] = state
	rm.state.LastUpdated = time.Now()

	// Update processed files count if needed
	if state.Status == "completed" {
		rm.state.ProcessedFiles++
	}
}
