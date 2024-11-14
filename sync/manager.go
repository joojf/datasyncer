package sync

import (
	"context"
	"datasyncer/types"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type SyncJob struct {
	SourcePath      string
	DestinationPath string
	FileInfo        types.FileInfo
}

type SyncManager struct {
	Providers map[types.CloudProvider]types.CloudStorage
	Logger    *types.Logger
	Notifier  *types.Notifier
	Recovery  *RecoveryManager
}

func NewSyncManager(logger *types.Logger, notifier *types.Notifier, recovery *RecoveryManager) *SyncManager {
	return &SyncManager{
		Providers: make(map[types.CloudProvider]types.CloudStorage),
		Logger:    logger,
		Notifier:  notifier,
		Recovery:  recovery,
	}
}

func (sm *SyncManager) Sync(ctx context.Context, opts types.SyncOptions) error {
	sourceProvider := sm.Providers[opts.SourceProvider]
	destProvider := sm.Providers[opts.DestinationProvider]

	if sourceProvider == nil || destProvider == nil {
		return fmt.Errorf("source or destination provider not configured")
	}

	files, err := sourceProvider.ListFiles(ctx, opts.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to list source files: %v", err)
	}

	jobs := make(chan SyncJob, len(files))
	var wg sync.WaitGroup

	for i := 0; i < opts.Parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if err := sm.processFile(ctx, job, sourceProvider, destProvider, opts); err != nil {
					sm.Logger.LogError(fmt.Sprintf("Failed to sync file %s: %v", job.SourcePath, err))
				}
			}
		}()
	}

	for _, file := range files {
		destPath := filepath.Join(opts.DestinationPath, filepath.Base(file.Path))
		jobs <- SyncJob{
			SourcePath:      file.Path,
			DestinationPath: destPath,
			FileInfo:        file,
		}
	}

	close(jobs)
	wg.Wait()

	sm.Notifier.SendNotification("Sync Completed", fmt.Sprintf("Synchronized %d files", len(files)))

	return nil
}

func (sm *SyncManager) processFile(ctx context.Context, job SyncJob, source, dest types.CloudStorage, opts types.SyncOptions) error {
	rm := sm.Recovery
	fileState, exists := rm.GetFileState(job.SourcePath)

	if exists && fileState.Status == "completed" {
		return nil
	}

	if exists && fileState.Attempts >= sm.Recovery.maxAttempts {
		sm.Logger.LogError(fmt.Sprintf("max retry attempts exceeded for file: %s", job.SourcePath))
		return fmt.Errorf("max retry attempts exceeded for file: %s", job.SourcePath)
	}

	fileState = FileState{
		Path:         job.SourcePath,
		Size:         job.FileInfo.Size,
		LastModified: job.FileInfo.LastModified,
		ETag:         job.FileInfo.ETag,
		Status:       "in_progress",
		Attempts:     fileState.Attempts + 1,
	}
	rm.UpdateFileState(fileState)

	err := sm.transferFile(ctx, job, source, dest)
	if err != nil {
		sm.Logger.LogError(fmt.Sprintf("Failed to transfer file %s: %v", job.SourcePath, err))

		fileState.Status = "failed"
		rm.UpdateFileState(fileState)
		return err
	}

	fileState.Status = "completed"
	rm.UpdateFileState(fileState)

	sm.Logger.LogInfo(fmt.Sprintf("Transferred %d bytes from %s to %s", job.FileInfo.Size, job.SourcePath, job.DestinationPath))

	return nil
}

func (sm *SyncManager) handleConflict(ctx context.Context, job SyncJob, destInfo types.FileInfo, source, dest types.CloudStorage, opts types.SyncOptions) error {
	switch opts.ConflictResolution {
	case "overwrite":
		return sm.transferFile(ctx, job, source, dest)

	case "skip":
		sm.Logger.LogInfo(fmt.Sprintf("Skipping existing file: %s", job.SourcePath))
		return nil

	case "archive":
		timestamp := time.Now().Format("20060102150405")
		archivePath := fmt.Sprintf("%s.%s", job.DestinationPath, timestamp)

		archiveJob := SyncJob{
			SourcePath:      job.DestinationPath,
			DestinationPath: archivePath,
			FileInfo:        destInfo,
		}

		if err := sm.transferFile(ctx, archiveJob, dest, dest); err != nil {
			return err
		}
		return sm.transferFile(ctx, job, source, dest)

	default:
		return fmt.Errorf("unknown conflict resolution strategy: %s", opts.ConflictResolution)
	}
}

func (sm *SyncManager) transferFile(ctx context.Context, job SyncJob, source, dest types.CloudStorage) error {
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, filepath.Base(job.SourcePath))

	if err := source.DownloadFile(ctx, job.SourcePath, tempFile); err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer os.Remove(tempFile)

	var lastErr error
	for i := 0; i < 3; i++ {
		if err := dest.UploadFile(ctx, tempFile, job.DestinationPath); err != nil {
			lastErr = err
			time.Sleep(time.Second * time.Duration(i+1))
			continue
		}
		return nil
	}

	return fmt.Errorf("failed to upload file after 3 attempts: %v", lastErr)
}
