package docker

import (
	"context"
	"github.com/docker/docker/client"
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

	_, err = c.DiscoverAssets(context.Background())
	if err != nil {
		panic(err)
	}
}
