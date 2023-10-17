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

package scanestimation

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v4"

	"github.com/openclarity/vmclarity/api/models"
	"github.com/openclarity/vmclarity/pkg/orchestrator/provider/common"
	familiestypes "github.com/openclarity/vmclarity/pkg/shared/families/types"
	"github.com/openclarity/vmclarity/pkg/shared/utils"
)

type MarketOption string

const (
	MarketOptionSpot     MarketOption = "Spot"
	MarketOptionOnDemand MarketOption = "OnDemand"
)

type ScanEstimator struct {
	priceFetcher PriceFetcherImpl
}

type EstimateAssetScanParams struct {
	SourceRegion           string
	DestRegion             string
	DiskStorageAccountType armcompute.DiskStorageAccountTypes
	StorageAccountType     armcompute.StorageAccountTypes
	ScannerVMSize          armcompute.VirtualMachineSizeTypes
	ScannerOSDiskSizeGB    int64
	Stats                  models.AssetScanStats
	Asset                  *models.Asset
	AssetScanTemplate      *models.AssetScanTemplate
}

func New() *ScanEstimator {
	return &ScanEstimator{
		priceFetcher: PriceFetcherImpl{client: http.Client{}},
	}
}

const (
	SecondsInAMonth = 86400 * 30
	SecondsInAnHour = 60 * 60
)

type recipeResource string

const (
	Snapshot      recipeResource = "Snapshot"
	ScannerVM     recipeResource = "ScannerVM"
	DataDisk      recipeResource = "DataDisk"
	ScannerOSDisk recipeResource = "ScannerOSDisk"
	DataTransfer  recipeResource = "DataTransfer"
	BlobStorage   recipeResource = "BlobStorage"
)

// scanSizesGB represents the memory sizes on the machines that the tests were taken on.
var scanSizesGB = []float64{0.01, 1.652, 4.559}

// FamilyScanDurationsMap Calculate the logarithmic fit of each family base on static measurements of family scan duration in seconds per scanSizesGB value.
// The tests were made on a Standard_D2s_v3 virtual machine with Standard SSD LRS os disk (30 GB)
// The times correspond to the scan size values in scanSizesGB.
// TODO add infoFinder family stats.
// nolint:gomnd

var jobCreationTime = common.MustLogarithmicFit(scanSizesGB, []float64{0.01, 1860, 2460})

var FamilyScanDurationsMap = map[familiestypes.FamilyType]*common.LogarithmicFormula{
	familiestypes.SBOM:             common.MustLogarithmicFit(scanSizesGB, []float64{0.01, 16, 17}),
	familiestypes.Vulnerabilities:  common.MustLogarithmicFit(scanSizesGB, []float64{0.01, 4, 10}), // TODO check time with no sbom scan
	familiestypes.Secrets:          common.MustLogarithmicFit(scanSizesGB, []float64{0.01, 420, 780}),
	familiestypes.Exploits:         common.MustLogarithmicFit(scanSizesGB, []float64{0, 0, 0}),
	familiestypes.Rootkits:         common.MustLogarithmicFit(scanSizesGB, []float64{0, 0, 0}),
	familiestypes.Misconfiguration: common.MustLogarithmicFit(scanSizesGB, []float64{0.01, 6, 7}),
	familiestypes.Malware:          common.MustLogarithmicFit(scanSizesGB, []float64{0.01, 900, 1140}),
}

// nolint:cyclop
func (s *ScanEstimator) EstimateAssetScan(ctx context.Context, params EstimateAssetScanParams) (*models.Estimation, error) {
	var sourceSnapshotMonthlyCost float64
	var err error

	if params.AssetScanTemplate == nil || params.AssetScanTemplate.ScanFamiliesConfig == nil {
		return nil, fmt.Errorf("scan families config was not provided")
	}
	familiesConfig := params.AssetScanTemplate.ScanFamiliesConfig

	// Get scan size and scan duration using previous stats and asset info.
	scanSizeMB, err := common.GetScanSize(params.Stats, params.Asset)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan size: %w", err)
	}
	scanSizeGB := float64(scanSizeMB) / common.MBInGB
	scanDurationSec := common.GetScanDuration(params.Stats, familiesConfig, scanSizeMB, FamilyScanDurationsMap)

	marketOption := MarketOptionOnDemand
	if params.AssetScanTemplate.UseSpotInstances() {
		marketOption = MarketOptionSpot
	}

	sourceRegion := params.SourceRegion
	destRegion := params.DestRegion
	// The data disk that is created from the snapshot.
	dataDiskType := params.DiskStorageAccountType
	scannerOSDiskType := params.DiskStorageAccountType
	jobCreationTimeSec := jobCreationTime.Evaluate(scanSizeGB)
	// The approximate amount of time that a resource is up before the scan starts (during job creation)
	idleRunTime := jobCreationTimeSec / 2
	scannerVMSize := params.ScannerVMSize
	scannerOSDiskSizeGB := params.ScannerOSDiskSizeGB

	snapshotStorageAccountType, err := getSnapshotTypeFromDiskType(params.DiskStorageAccountType)
	if err != nil {
		return nil, err
	}

	// Get relevant current prices from Azure price list API
	// Fetch the dest snapshot monthly cost.
	destSnapshotMonthlyCost, err := s.priceFetcher.GetSnapshotGBPerMonthCost(destRegion, snapshotStorageAccountType)
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly cost for destination snapshot: %w", err)
	}

	// Fetch the scanner vm hourly cost.
	scannerPerHourCost, err := s.priceFetcher.GetInstancePerHourCost(destRegion, scannerVMSize, marketOption)
	if err != nil {
		return nil, fmt.Errorf("failed to get scanner per hour cost: %w", err)
	}

	// Fetch the scanner os disk monthly cost.
	scannerOSDiskMonthlyCost, err := s.priceFetcher.GetManagedDiskMonthlyCost(destRegion, scannerOSDiskType, scannerOSDiskSizeGB)
	if err != nil {
		return nil, fmt.Errorf("failed to get os disk monthly cost per GB: %w", err)
	}

	// Fetch the monthly cost of the disk that was created from the snapshot.
	// We assume that the data disk size will be the same as the os disk size.
	diskFromSnapshotMonthlyCost, err := s.priceFetcher.GetManagedDiskMonthlyCost(destRegion, dataDiskType, scannerOSDiskSizeGB)
	if err != nil {
		return nil, fmt.Errorf("failed to get data disk monthly cost per GB: %w", err)
	}

	dataTransferCost := 0.0
	sourceSnapshotCost := 0.0
	blobStorageCost := 0.0
	if sourceRegion != destRegion {
		// if the scanner is in a different region than the scanned asset, we have another snapshot created in the
		// source region.
		sourceSnapshotMonthlyCost, err = s.priceFetcher.GetSnapshotGBPerMonthCost(sourceRegion, snapshotStorageAccountType)
		if err != nil {
			return nil, fmt.Errorf("failed to get source snapshot monthly cost: %w", err)
		}

		sourceSnapshotCost = sourceSnapshotMonthlyCost * ((idleRunTime + float64(scanDurationSec)) / SecondsInAMonth) * scanSizeGB

		// Fetch the data transfer cost per GB (if source and dest regions are the same, this will be 0).
		dataTransferCostPerGB, err := s.priceFetcher.GetDataTransferPerGBCost(destRegion)
		if err != nil {
			return nil, fmt.Errorf("failed to get data transfer cost per GB: %w", err)
		}

		dataTransferCost = dataTransferCostPerGB * scanSizeGB

		// when moving a snapshot into another region, the snapshot is copied into a blob storage.
		blobStoragePerGB, err := s.priceFetcher.GetBlobStoragePerGBCost(destRegion, params.StorageAccountType)

		blobStorageCost = blobStoragePerGB * scanSizeGB
	}

	destSnapshotCost := destSnapshotMonthlyCost * ((idleRunTime + float64(scanDurationSec)) / SecondsInAMonth) * scanSizeGB
	volumeFromSnapshotCost := diskFromSnapshotMonthlyCost * ((idleRunTime + float64(scanDurationSec)) / SecondsInAMonth) * scanSizeGB
	scannerCost := scannerPerHourCost * ((idleRunTime + float64(scanDurationSec)) / SecondsInAnHour)
	scannerRootVolumeCost := scannerOSDiskMonthlyCost * ((idleRunTime + float64(scanDurationSec)) / SecondsInAMonth) * float64(scannerOSDiskSizeGB)

	jobTotalCost := sourceSnapshotCost + volumeFromSnapshotCost + scannerCost + scannerRootVolumeCost + dataTransferCost + destSnapshotCost + blobStorageCost

	// Create the Estimation object base on the calculated data.
	costBreakdown := []models.CostBreakdownComponent{
		{
			Cost:      float32(destSnapshotCost),
			Operation: fmt.Sprintf("%v-%v", Snapshot, destRegion),
		},
		{
			Cost:      float32(scannerCost),
			Operation: string(ScannerVM),
		},
		{
			Cost:      float32(volumeFromSnapshotCost),
			Operation: string(DataDisk),
		},
		{
			Cost:      float32(scannerRootVolumeCost),
			Operation: string(ScannerOSDisk),
		},
	}
	if sourceRegion != destRegion {
		costBreakdown = append(costBreakdown, []models.CostBreakdownComponent{
			{
				Cost:      float32(sourceSnapshotCost),
				Operation: fmt.Sprintf("%v-%v", Snapshot, sourceRegion),
			},
			{
				Cost:      float32(dataTransferCost),
				Operation: string(DataTransfer),
			},
			{
				Cost:      float32(blobStorageCost),
				Operation: string(BlobStorage),
			},
		}...)
	}

	estimation := models.Estimation{
		Cost:          utils.PointerTo(float32(jobTotalCost)),
		CostBreakdown: &costBreakdown,
		Size:          utils.PointerTo(int(scanSizeGB)),
		Duration:      utils.PointerTo(int(scanDurationSec)),
	}

	return &estimation, nil
}

func getSnapshotTypeFromDiskType(diskType armcompute.DiskStorageAccountTypes) (armcompute.SnapshotStorageAccountTypes, error) {
	switch diskType {
	case armcompute.DiskStorageAccountTypesStandardSSDLRS:
		return armcompute.SnapshotStorageAccountTypesStandardLRS, nil
	default:
		return "", fmt.Errorf("unsupported disk type: %v", diskType)
	}
}