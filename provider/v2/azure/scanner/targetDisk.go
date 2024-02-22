// Copyright © 2023 Cisco Systems, Inc. and its affiliates.
// All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package scanner

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"github.com/openclarity/vmclarity/core/to"
	"github.com/openclarity/vmclarity/provider"
	"github.com/openclarity/vmclarity/provider/v2/azure/common"
)

var (
	DiskEstimateProvisionTime = 2 * time.Minute
)

func volumeNameFromJobConfig(config *provider.ScanJobConfig) string {
	return "targetvolume-" + config.AssetScanID
}

func (s *Scanner) ensureManagedDiskFromSnapshot(ctx context.Context, config *provider.ScanJobConfig, snapshot armcompute.Snapshot) (armcompute.Disk, error) {
	volumeName := volumeNameFromJobConfig(config)

	volumeRes, err := s.DisksClient.Get(ctx, s.Config.ScannerResourceGroup, volumeName, nil)
	if err == nil {
		if *volumeRes.Disk.Properties.ProvisioningState != provisioningStateSucceeded {
			return volumeRes.Disk, provider.RetryableErrorf(DiskEstimateProvisionTime, "volume is not ready yet, provisioning state: %s", *volumeRes.Disk.Properties.ProvisioningState)
		}

		return volumeRes.Disk, nil
	}

	notFound, err := common.HandleAzureRequestError(err, "getting volume %s", volumeName)
	if !notFound {
		return armcompute.Disk{}, err
	}

	_, err = s.DisksClient.BeginCreateOrUpdate(ctx, s.Config.ScannerResourceGroup, volumeName, armcompute.Disk{
		Location: to.Ptr(s.Config.ScannerLocation),
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(armcompute.DiskStorageAccountTypesStandardSSDLRS),
		},
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionCopy),
				SourceResourceID: snapshot.ID,
			},
		},
	}, nil)
	if err != nil {
		_, err := common.HandleAzureRequestError(err, "creating disk %s", volumeName)
		return armcompute.Disk{}, err
	}

	return armcompute.Disk{}, provider.RetryableErrorf(DiskEstimateProvisionTime, "disk creating")
}

func (s *Scanner) ensureManagedDiskFromSnapshotInDifferentRegion(ctx context.Context, config *provider.ScanJobConfig, snapshot armcompute.Snapshot) (armcompute.Disk, error) {
	blobURL, err := s.ensureBlobFromSnapshot(ctx, config, snapshot)
	if err != nil {
		return armcompute.Disk{}, fmt.Errorf("failed to ensure blob from snapshot: %w", err)
	}

	volumeName := volumeNameFromJobConfig(config)

	volumeRes, err := s.DisksClient.Get(ctx, s.Config.ScannerResourceGroup, volumeName, nil)
	if err == nil {
		if *volumeRes.Disk.Properties.ProvisioningState != provisioningStateSucceeded {
			return volumeRes.Disk, provider.RetryableErrorf(DiskEstimateProvisionTime, "volume is not ready yet, provisioning state: %s", *volumeRes.Disk.Properties.ProvisioningState)
		}

		return volumeRes.Disk, nil
	}

	notFound, err := common.HandleAzureRequestError(err, "getting volume %s", volumeName)
	if !notFound {
		return armcompute.Disk{}, err
	}

	_, err = s.DisksClient.BeginCreateOrUpdate(ctx, s.Config.ScannerResourceGroup, volumeName, armcompute.Disk{
		Location: to.Ptr(s.Config.ScannerLocation),
		SKU: &armcompute.DiskSKU{
			Name: to.Ptr(armcompute.DiskStorageAccountTypesStandardSSDLRS),
		},
		Properties: &armcompute.DiskProperties{
			CreationData: &armcompute.CreationData{
				CreateOption:     to.Ptr(armcompute.DiskCreateOptionImport),
				SourceURI:        to.Ptr(blobURL),
				StorageAccountID: to.Ptr(fmt.Sprintf("subscriptions/%s/resourceGroups/%s/providers/Microsoft.Storage/storageAccounts/%s", s.Config.SubscriptionID, s.Config.ScannerResourceGroup, s.Config.ScannerStorageAccountName)),
			},
		},
	}, nil)
	if err != nil {
		_, err := common.HandleAzureRequestError(err, "creating disk %s", volumeName)
		return armcompute.Disk{}, err
	}
	return armcompute.Disk{}, provider.RetryableErrorf(DiskEstimateProvisionTime, "disk creating")
}
