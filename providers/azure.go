package providers

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"datasyncer/types"

	"github.com/Azure/azure-storage-blob-go/azblob"
)

type AzureProvider struct {
	containerURL azblob.ContainerURL
	credential   azblob.Credential
}

func NewAzureProvider(accountName, accountKey, containerName string) *AzureProvider {
	return &AzureProvider{}
}

func (a *AzureProvider) Authenticate(ctx context.Context) error {
	credential, err := azblob.NewSharedKeyCredential(
		os.Getenv("AZURE_STORAGE_ACCOUNT"),
		os.Getenv("AZURE_STORAGE_ACCESS_KEY"))
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %v", err)
	}

	pipeline := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s",
		os.Getenv("AZURE_STORAGE_ACCOUNT"),
		os.Getenv("AZURE_STORAGE_CONTAINER")))

	a.containerURL = azblob.NewContainerURL(*URL, pipeline)
	a.credential = credential

	return nil
}

func (a *AzureProvider) ListFiles(ctx context.Context, path string) ([]types.FileInfo, error) {
	var files []types.FileInfo

	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := a.containerURL.ListBlobsFlatSegment(ctx, marker, azblob.ListBlobsSegmentOptions{
			Prefix: path,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to list blobs: %v", err)
		}

		marker = listBlob.NextMarker

		for _, blobInfo := range listBlob.Segment.BlobItems {
			files = append(files, types.FileInfo{
				Path:         blobInfo.Name,
				Size:         *blobInfo.Properties.ContentLength,
				LastModified: blobInfo.Properties.LastModified,
				ETag:         string(blobInfo.Properties.Etag),
			})
		}
	}

	return files, nil
}

func (a *AzureProvider) UploadFile(ctx context.Context, localPath, remotePath string) error {
	blobURL := a.containerURL.NewBlockBlobURL(remotePath)

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer file.Close()

	_, err = azblob.UploadFileToBlockBlob(ctx, file, blobURL, azblob.UploadToBlockBlobOptions{
		BlockSize:   4 * 1024 * 1024, // 4MB block size
		Parallelism: 16,              // 16 parallel operations
	})
	if err != nil {
		return fmt.Errorf("failed to upload file: %v", err)
	}

	return nil
}

func (a *AzureProvider) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	blobURL := a.containerURL.NewBlockBlobURL(remotePath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %v", err)
	}

	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer file.Close()

	response, err := blobURL.Download(ctx, 0, azblob.CountToEnd, azblob.BlobAccessConditions{}, false, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return fmt.Errorf("failed to download blob: %v", err)
	}

	bodyStream := response.Body(azblob.RetryReaderOptions{MaxRetryRequests: 3})
	defer bodyStream.Close()

	_, err = io.Copy(file, bodyStream)
	if err != nil {
		return fmt.Errorf("failed to write file: %v", err)
	}

	return nil
}

func (a *AzureProvider) GetFileInfo(ctx context.Context, path string) (types.FileInfo, error) {
	blobURL := a.containerURL.NewBlockBlobURL(path)

	props, err := blobURL.GetProperties(ctx, azblob.BlobAccessConditions{}, azblob.ClientProvidedKeyOptions{})
	if err != nil {
		return types.FileInfo{}, fmt.Errorf("failed to get blob properties: %v", err)
	}

	return types.FileInfo{
		Path:         path,
		Size:         props.ContentLength(),
		LastModified: props.LastModified(),
		ETag:         string(props.ETag()),
	}, nil
}

func (a *AzureProvider) DeleteFile(ctx context.Context, path string) error {
	blobURL := a.containerURL.NewBlockBlobURL(path)

	_, err := blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	if err != nil {
		return fmt.Errorf("failed to delete blob: %v", err)
	}

	return nil
}
