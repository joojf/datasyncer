package providers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"datasyncer/types"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type AWSS3Provider struct {
	client *s3.Client
	bucket string
}

func NewAWSS3Provider(bucket string) *AWSS3Provider {
	return &AWSS3Provider{
		bucket: bucket,
	}
}

func (a *AWSS3Provider) Authenticate(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("unable to load SDK config: %v", err)
	}

	a.client = s3.NewFromConfig(cfg)
	return nil
}

func (a *AWSS3Provider) ListFiles(ctx context.Context, path string) ([]types.FileInfo, error) {
	var files []types.FileInfo

	input := &s3.ListObjectsV2Input{
		Bucket: &a.bucket,
		Prefix: &path,
	}

	paginator := s3.NewListObjectsV2Paginator(a.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %v", err)
		}

		for _, obj := range page.Contents {
			files = append(files, types.FileInfo{
				Path:         *obj.Key,
				Size:         *obj.Size,
				LastModified: *obj.LastModified,
				ETag:         *obj.ETag,
			})
		}
	}

	return files, nil
}

func (a *AWSS3Provider) UploadFile(ctx context.Context, localPath, remotePath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	_, err = a.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &a.bucket,
		Key:    &remotePath,
		Body:   file,
	})

	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	return nil
}

func (a *AWSS3Provider) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	result, err := a.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &a.bucket,
		Key:    &remotePath,
	})
	if err != nil {
		return fmt.Errorf("failed to download file: %v", err)
	}
	defer result.Body.Close()

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, result.Body)
	if err != nil {
		return fmt.Errorf("failed to save file: %v", err)
	}

	return nil
}

func (a *AWSS3Provider) GetFileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	result, err := a.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &a.bucket,
		Key:    &path,
	})
	if err != nil {
		return types.FileInfo{}, fmt.Errorf("failed to get file info: %v", err)
	}

	return types.FileInfo{
		Path:         path,
		Size:         *result.ContentLength,
		LastModified: *result.LastModified,
		ETag:         *result.ETag,
	}, nil
}

func (a *AWSS3Provider) DeleteFile(ctx context.Context, path string) error {
	_, err := a.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &a.bucket,
		Key:    &path,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %v", err)
	}
	return nil
}
