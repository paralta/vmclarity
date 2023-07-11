package docker

import (
	"context"
	"github.com/docker/docker/client"
	"github.com/openclarity/vmclarity/api/models"
	"github.com/openclarity/vmclarity/runtime_scan/pkg/provider"
	"testing"
)

func TestClient(t *testing.T) {

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(err)
	}

	c := Client{
		dockerClient: dockerClient,
		config: &Config{
			ScannerImage: "ghcr.io/openclarity/vmclarity-cli:latest",
		},
	}

	assets, err := c.DiscoverAssets(context.Background())
	if err != nil {
		panic(err)
	}

	for _, asset := range assets {
		jobConfig := provider.ScanJobConfig{
			ScannerImage:     "ghcr.io/openclarity/vmclarity-cli:latest",
			ScannerCLIConfig: "",
			VMClarityAddress: "http://host.docker.internal:8888/api",
			ScanMetadata: provider.ScanMetadata{
				AssetScanID: "803716a3-a97a-45b0-966f-5dd7ccfc2df4",
			},
			ScannerInstanceCreationConfig: models.ScannerInstanceCreationConfig{},
			Asset: models.Asset{
				AssetInfo: &asset,
			},
		}
		err = c.RunAssetScan(context.Background(), &jobConfig)
		if err != nil {
			panic(err)
		}

		err = c.RemoveAssetScan(context.Background(), &jobConfig)
		if err != nil {
			panic(err)
		}
	}

}
