package providers

import (
	"datasyncer/types"
	"fmt"
)

// type ProviderConfig struct {
// 	Type          types.CloudProvider
// 	Bucket        string
// 	ProjectID     string // For GCP
// 	AccountName   string // For Azure
// 	AccountKey    string // For Azure
// 	ContainerName string // For Azure
// }

func CreateProvider(config types.ProviderConfig) (types.CloudStorage, error) {
	switch config.Type {
	case types.AWS:
		return NewAWSS3Provider(config.Bucket), nil

	case types.GCP:
		if config.ProjectID == "" {
			return nil, fmt.Errorf("project ID is required for GCP provider")
		}
		return NewGCPProvider(config.Bucket, config.ProjectID), nil

	case types.AZURE:
		if config.AccountName == "" || config.AccountKey == "" || config.ContainerName == "" {
			return nil, fmt.Errorf("account name, account key, and container name are required for Azure provider")
		}
		return NewAzureProvider(config.AccountName, config.AccountKey, config.ContainerName), nil

	default:
		return nil, fmt.Errorf("unsupported provider type: %s", config.Type)
	}
}
