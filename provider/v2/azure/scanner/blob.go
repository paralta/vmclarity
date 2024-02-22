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
	"log"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"

	"github.com/openclarity/vmclarity/core/to"
	"github.com/openclarity/vmclarity/provider"
	"github.com/openclarity/vmclarity/provider/v2/azure/common"
)

var (
	estimatedBlobCopyTime = 2 * time.Minute
	snapshotSASAccessSeconds = 3600
)

func blobNameFromJobConfig(config *provider.ScanJobConfig) string {
	return config.AssetScanID + ".vhd"
}

func (s *Scanner) blobURLFromBlobName(blobName string) string {
	return fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", s.Config.ScannerStorageAccountName, s.Config.ScannerStorageContainerName, blobName)
}

func (s *Scanner) ensureBlobFromSnapshot(ctx context.Context, config *provider.ScanJobConfig, snapshot armcompute.Snapshot) (string, error) {
	blobName := blobNameFromJobConfig(config)
	blobURL := s.blobURLFromBlobName(blobName)
	blobClient, err := blob.NewClient(blobURL, s.Cred, nil)
	if err != nil {
		return blobURL, provider.FatalErrorf("failed to init blob client: %w", err)
	}

	getMetadata, err := blobClient.GetProperties(ctx, nil)
	if err == nil {
		copyStatus := *getMetadata.CopyStatus
		if copyStatus != blob.CopyStatusTypeSuccess {
			log.Print("blob is still copying, status is ", copyStatus)
			return blobURL, provider.RetryableErrorf(estimatedBlobCopyTime, "blob is still copying")
		}

		revokepoller, err := s.SnapshotsClient.BeginRevokeAccess(ctx, s.Config.ScannerResourceGroup, *snapshot.Name, nil)
		if err != nil {
			_, err := common.HandleAzureRequestError(err, "revoking SAS access for snapshot %s", *snapshot.Name)
			return blobURL, err
		}
		_, err = revokepoller.PollUntilDone(ctx, nil)
		if err != nil {
			_, err := common.HandleAzureRequestError(err, "waiting for SAS access to be revoked for snapshot %s", *snapshot.Name)
			return blobURL, err
		}

		return blobURL, nil
	}

	notFound, err := common.HandleAzureRequestError(err, "getting blob %s", blobName)
	if !notFound {
		return blobURL, err
	}

	// NOTE(sambetts) Granting SAS access to a snapshot must be done
	// atomically with starting the CopyFromUrl Operation because
	// GrantAccess only provides the URL once, and we don't want to store
	// it.
	poller, err := s.SnapshotsClient.BeginGrantAccess(ctx, s.Config.ScannerResourceGroup, *snapshot.Name, armcompute.GrantAccessData{
		Access:            to.Ptr(armcompute.AccessLevelRead),
		DurationInSeconds: to.Ptr[int32](int32(snapshotSASAccessSeconds)),
	}, nil)
	if err != nil {
		_, err := common.HandleAzureRequestError(err, "granting SAS access to snapshot %s", *snapshot.Name)
		return blobURL, err
	}

	res, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		_, err := common.HandleAzureRequestError(err, "waiting for SAS access to snapshot %s be granted", *snapshot.Name)
		return blobURL, err
	}

	accessURL := *res.AccessURI.AccessSAS

	_, err = blobClient.StartCopyFromURL(ctx, accessURL, nil)
	if err != nil {
		_, err := common.HandleAzureRequestError(err, "starting copy from URL operation for blob %s", blobName)
		return blobURL, err
	}

	return blobURL, provider.RetryableErrorf(estimatedBlobCopyTime, "blob copy from url started")
}
