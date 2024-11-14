package types

import (
	"context"
	"fmt"
	"os"
	"time"
)

type CloudProvider string

const (
	AWS   CloudProvider = "aws"
	GCP   CloudProvider = "gcp"
	AZURE CloudProvider = "azure"
)

type FileInfo struct {
	Path         string
	Size         int64
	LastModified time.Time
	ETag         string
}

type CloudStorage interface {
	Authenticate(ctx context.Context) error

	ListFiles(ctx context.Context, path string) ([]FileInfo, error)
	UploadFile(ctx context.Context, localPath, remotePath string) error
	DownloadFile(ctx context.Context, remotePath, localPath string) error
	DeleteFile(ctx context.Context, path string) error

	GetFileInfo(ctx context.Context, path string) (FileInfo, error)
}

type SyncOptions struct {
	SourceProvider      CloudProvider
	DestinationProvider CloudProvider
	SourcePath          string
	DestinationPath     string
	Parallel            int
	ConflictResolution  string
	IncrementalSync     bool
}

type Logger struct {
	LogFile string
}

func (l *Logger) LogError(message string) {
	// Implementation for error logging
	// For now, just print to stderr
	fmt.Fprintf(os.Stderr, "[ERROR] %s\n", message)
}

func (l *Logger) LogInfo(message string) {
	// Implementation for info logging
	// For now, just print to stdout
	fmt.Printf("[INFO] %s\n", message)
}

type Notifier struct {
	EmailConfig EmailConfig
}

func (n *Notifier) SendNotification(title, message string) {
	// Implementation for sending notifications
	// For now, just print to stdout
	fmt.Printf("[NOTIFICATION] %s: %s\n", title, message)
}

type EmailConfig struct {
	SMTPServer string
	Port       int
	Username   string
	Password   string
	FromEmail  string
}

type ProviderConfig struct {
	Type          CloudProvider
	Bucket        string
	ProjectID     string // For GCP
	AccountName   string // For Azure
	AccountKey    string // For Azure
	ContainerName string // For Azure
}
