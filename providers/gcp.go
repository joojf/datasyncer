package providers

import (
	"context"
	"datasyncer/types"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GCPProvider struct {
	client    *storage.Client
	bucket    string
	projectID string
}

func NewGCPProvider(bucket, projectID string) *GCPProvider {
	return &GCPProvider{
		bucket:    bucket,
		projectID: projectID,
	}
}

func (g *GCPProvider) Authenticate(ctx context.Context) error {
	var err error
	g.client, err = storage.NewClient(ctx, option.WithCredentialsFile("gcp-credentials.json"))
	if err != nil {
		return fmt.Errorf("failed to create GCP client: %v", err)
	}
	return nil
}

func (g *GCPProvider) ListFiles(ctx context.Context, path string) ([]types.FileInfo, error) {
	var files []types.FileInfo
	bucket := g.client.Bucket(g.bucket)
	it := bucket.Objects(ctx, &storage.Query{Prefix: path})

	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("error iterating objects: %v", err)
		}

		files = append(files, types.FileInfo{
			Path:         attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
			ETag:         attrs.Etag,
		})
	}

	return files, nil
}

func (g *GCPProvider) UploadFile(ctx context.Context, localPath, remotePath string) error {
	bucket := g.client.Bucket(g.bucket)
	obj := bucket.Object(remotePath)

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer file.Close()

	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, file); err != nil {
		writer.Close()
		return fmt.Errorf("failed to copy data to GCS: %v", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close writer: %v", err)
	}

	return nil
}

func (g *GCPProvider) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	bucket := g.client.Bucket(g.bucket)
	obj := bucket.Object(remotePath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer file.Close()

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create reader: %v", err)
	}
	defer reader.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to copy data from GCS: %v", err)
	}

	return nil
}

func (g *GCPProvider) GetFileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	bucket := g.client.Bucket(g.bucket)
	obj := bucket.Object(path)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		return types.FileInfo{}, fmt.Errorf("failed to get object attributes: %v", err)
	}

	return types.FileInfo{
		Path:         attrs.Name,
		Size:         attrs.Size,
		LastModified: attrs.Updated,
		ETag:         attrs.Etag,
	}, nil
}

func (g *GCPProvider) DeleteFile(ctx context.Context, path string) error {
	bucket := g.client.Bucket(g.bucket)
	obj := bucket.Object(path)

	if err := obj.Delete(ctx); err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}
