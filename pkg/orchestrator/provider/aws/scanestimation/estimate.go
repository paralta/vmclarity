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

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	kubeclarityUtils "github.com/openclarity/kubeclarity/shared/pkg/utils"

	"github.com/openclarity/vmclarity/api/models"
	familiestypes "github.com/openclarity/vmclarity/pkg/shared/families/types"
	"github.com/openclarity/vmclarity/pkg/shared/utils"
)

type ScanEstimator struct {
	offerFileFetcher OfferFileFetcher
}

type EstimateAssetScanParams struct {
	SourceRegion            string
	DestRegion              string
	ScannerVolumeType       ec2types.VolumeType
	FromSnapshotVolumeType  ec2types.VolumeType
	ScannerInstanceType     ec2types.InstanceType
	JobCreationTimeSec      float64
	ScannerRootVolumeSizeGB int
	Stats                   models.AssetScanStats
	Asset                   *models.Asset
	AssetScanTemplate       *models.AssetScanTemplate
}

func New(pricingClient *pricing.Client) *ScanEstimator {
	return &ScanEstimator{
		offerFileFetcher: &OfferFileFetcherImpl{pricingClient: pricingClient},
	}
}

const (
	SecondsInAMonth = 86400 * 30
	SecondsInAnHour = 60 * 60
)

type recipeResource string

const (
	SourceSnapshot      recipeResource = "SourceSnapshot"
	DestinationSnapshot recipeResource = "DestinationSnapshot"
	ScannerInstance     recipeResource = "ScannerInstance"
	VolumeFromSnapshot  recipeResource = "VolumeFromSnapshot"
	ScannerRootVolume   recipeResource = "ScannerRootVolume"
	DataTransfer        recipeResource = "DataTransfer"
)

// Static times from lab tests of family scan duration in seconds per GB
var familyScanDurationPerGBMap = map[familiestypes.FamilyType]float64{
	familiestypes.SBOM:             4.5,
	familiestypes.Vulnerabilities:  1, // TODO check time with no sbom scan
	familiestypes.Secrets:          300,
	familiestypes.Exploits:         0,
	familiestypes.Rootkits:         0,
	familiestypes.Misconfiguration: 1,
	familiestypes.Malware:          360,
}

// Reserved Instances are not physical instances, but rather a billing discount that is applied to the running On-Demand Instances in your account.
// The On-Demand Instances must match certain specifications of the Reserved Instances in order to benefit from the billing discount.
// A decade after launching Reserved Instances (RIs), Amazon Web Services (AWS) introduced Savings Plans as a more flexible alternative to RIs. AWS Savings Plans are not meant to replace Reserved Instances; they are complementary.

// We are not taking into account Reserved Instances (RIs) or Saving Plans (SPs) since we don't know the exact OnDemand configuration in order to launch them.
// In the future, we can let the user choose to use RI's or SP's as the scanner instances.

// BoxUsage explained: https://stackoverflow.com/questions/57005129/what-does-box-usage-mean-in-aws
var regionCodeToInstanceUsageType = map[string]string{
	"us-east-1":    "EBS:VolumeUsage", // n. virginia
	"us-east-2":    "USE2-BoxUsage",   // ohio
	"us-west-1":    "USW1-BoxUsage",   // n. california
	"us-west-2":    "USW2-BoxUsage",   // oregon
	"eu-central-1": "EUC1-BoxUsage",   // Frankfurt
	"eu-central-2": "EUC2-BoxUsage",   // Zurich
	"eu-south-1":   "EUS1-BoxUsage",   // Milan
	"eu-west-1":    "EUW1-BoxUsage",   // Ireland
	"eu-west-2":    "EUW2-BoxUsage",   // London
	"eu-west-3":    "EUW3-BoxUsage",   // Paris
}

var regionCodeToVolumeUsageType = map[string]string{
	"us-east-1":    "EBS:VolumeUsage",      // n. virginia
	"us-east-2":    "USE2-EBS:VolumeUsage", // ohio
	"us-west-1":    "USW1-EBS:VolumeUsage", // n. california
	"us-west-2":    "USW2-EBS:VolumeUsage", // oregon
	"eu-central-1": "EUC1-EBS:VolumeUsage", // Frankfurt
	"eu-central-2": "EUC2-EBS:VolumeUsage", // Zurich
	"eu-south-1":   "EUS1-EBS:VolumeUsage", // Milan
	"eu-west-1":    "EUW1-EBS:VolumeUsage", // Ireland
	"eu-west-2":    "EUW2-EBS:VolumeUsage", // London
	"eu-west-3":    "EUW3-EBS:VolumeUsage", // Paris
}

var regionCodeToSnapshotUsageType = map[string]string{
	"us-east-1":    "EBS:SnapshotUsage",      // n. virginia
	"us-east-2":    "USE2-EBS:SnapshotUsage", // ohio
	"us-west-1":    "USW1-EBS:SnapshotUsage", // n. california
	"us-west-2":    "USW2-EBS:SnapshotUsage", // oregon
	"eu-central-1": "EUC1-EBS:SnapshotUsage", // Frankfurt
	"eu-central-2": "EUC2-EBS:SnapshotUsage", // Zurich
	"eu-south-1":   "EUS1-EBS:SnapshotUsage", // Milan
	"eu-west-1":    "EUW1-EBS:SnapshotUsage", // Ireland
	"eu-west-2":    "EUW2-EBS:SnapshotUsage", // London
	"eu-west-3":    "EUW3-EBS:SnapshotUsage", // Paris
}

// TODO we can use this map instead of regionCodeToInstanceUsageType in order to identify our instance,
// and also we can use the sku/offer/rate numbers in order to get to the specific pricePerUnit in the json offer file (instead of taking the first value)
// The problem is that I am not sure if the rate codes and offer codes can change if there is a new price offering.
var regionCodeToRateCode = map[string]string{
	"us-east-1":    "QG5G45WKDWDDHTFV.JRTCKXETXF.6YS6EN2CT7",
	"us-east-2":    "B7KJQVXZZNDAS23N.JRTCKXETXF.6YS6EN2CT7",
	"us-west-1":    "D7EXY7CNAHW9BTHD.JRTCKXETXF.6YS6EN2CT7",
	"us-west-2":    "WE87HQHP89BK3AXK.JRTCKXETXF.6YS6EN2CT7",
	"eu-central-1": "",
	"eu-central-2": "",
	"eu-south-1":   "",
	"eu-west-1":    "",
	"eu-west-2":    "92TGCNQTRRAJJAS7.JRTCKXETXF.6YS6EN2CT7",
	"eu-west-3":    "",
}

// TODO - maxParallelScanners, spot, inputs, scanners (instead of family), scanners times, jobCreationTime, dataTransfer
func (s *ScanEstimator) EstimateAssetScan(ctx context.Context, params EstimateAssetScanParams) (*models.Estimation, error) {
	var sourceSnapshotMonthlyCost float64
	var err error

	if params.AssetScanTemplate == nil || params.AssetScanTemplate.ScanFamiliesConfig == nil {
		return nil, fmt.Errorf("scan families config was not provided")
	}
	familiesConfig := params.AssetScanTemplate.ScanFamiliesConfig

	// TODO assuming one input for now (ROOTFS) until we have inputs data in AssetScanTemplate
	input := familiestypes.Input{
		Input:     "/",
		InputType: string(kubeclarityUtils.ROOTFS),
	}

	// Get scan size and scan duration using previous stats and asset info.
	scanSizeMB, err := getScanSize(params.Stats, params.Asset, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan size: %v", err)
	}
	scanSizeGB := float64(scanSizeMB) / 1000
	scanDurationSec, err := getScanDuration(params.Stats, familiesConfig, scanSizeMB, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get scan duration: %v", err)
	}

	sourceRegion := params.SourceRegion
	destRegion := params.DestRegion
	fromSnapshotVolumeType := params.FromSnapshotVolumeType
	jobCreationTimeSec := params.JobCreationTimeSec
	scannerInstanceType := params.ScannerInstanceType
	timeForScanInSec := float64(scanDurationSec)
	scannerRootVolumeSizeGB := params.ScannerRootVolumeSizeGB
	scannerVolumeType := params.ScannerVolumeType

	// Get relevant current prices from AWS price list API
	if sourceRegion != destRegion {
		sourceSnapshotMonthlyCost, err = s.offerFileFetcher.GetSnapshotMonthlyCostPerGB(ctx, sourceRegion)
		if err != nil {
			return nil, fmt.Errorf("failed to get source snapshot monthly cost: %v", err)
		}
	}

	destSnapshotMonthlyCost, err := s.offerFileFetcher.GetSnapshotMonthlyCostPerGB(ctx, destRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to get dest snapshot monthly cost: %v", err)
	}

	scannerPerHourCost, err := s.offerFileFetcher.GetInstancePerHourCost(ctx, destRegion, scannerInstanceType)
	if err != nil {
		return nil, fmt.Errorf("failed to get scanner per hour cost: %v", err)
	}

	scannerRootVolumeMonthlyCost, err := s.offerFileFetcher.GetVolumeMonthlyCostPerGB(ctx, destRegion, scannerVolumeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume monthly cost per GB: %v", err)
	}

	dataTransferCostPerGB, err := s.offerFileFetcher.GetDataTransferCostPerGB(sourceRegion, destRegion)
	if err != nil {
		return nil, fmt.Errorf("failed to get data transfer cost per GB: %v", err)
	}

	volumeFromSnapshotMonthlyCost, err := s.offerFileFetcher.GetVolumeMonthlyCostPerGB(ctx, destRegion, fromSnapshotVolumeType)
	if err != nil {
		return nil, fmt.Errorf("failed to get volume monthly cost per GB: %v", err)
	}

	// Calculate the costs of all resources created during a scan.
	GBTransferred := scanSizeGB

	dataTransferCost := dataTransferCostPerGB * GBTransferred

	sourceSnapshotCost := sourceSnapshotMonthlyCost * ((jobCreationTimeSec + timeForScanInSec) / SecondsInAMonth) * scanSizeGB
	destSnapshotCost := destSnapshotMonthlyCost * ((jobCreationTimeSec + timeForScanInSec) / SecondsInAMonth) * scanSizeGB
	volumeFromSnapshotCost := volumeFromSnapshotMonthlyCost * ((jobCreationTimeSec + timeForScanInSec) / SecondsInAMonth) * scanSizeGB
	scannerCost := scannerPerHourCost * ((jobCreationTimeSec + timeForScanInSec) / SecondsInAnHour)
	scannerRootVolumeCost := scannerRootVolumeMonthlyCost * ((jobCreationTimeSec + timeForScanInSec) / SecondsInAMonth) * float64(scannerRootVolumeSizeGB)

	jobTotalCost := sourceSnapshotCost + volumeFromSnapshotCost + scannerCost + scannerRootVolumeCost + dataTransferCost + destSnapshotCost

	// Create the Estimation object base on the calculated data.
	estimation := models.Estimation{
		Cost: utils.PointerTo(float32(jobTotalCost)),
		Recipe: &[]models.RecipeComponent{
			{
				Cost:      float32(sourceSnapshotCost),
				Operation: string(SourceSnapshot),
			},
			{
				Cost:      float32(destSnapshotCost),
				Operation: string(DestinationSnapshot),
			},
			{
				Cost:      float32(scannerCost),
				Operation: string(ScannerInstance),
			},
			{
				Cost:      float32(volumeFromSnapshotCost),
				Operation: string(VolumeFromSnapshot),
			},
			{
				Cost:      float32(scannerRootVolumeCost),
				Operation: string(ScannerRootVolume),
			},
			{
				Cost:      float32(dataTransferCost),
				Operation: string(DataTransfer),
			},
		},
		Size: utils.PointerTo(int(scanSizeGB)),
		Time: utils.PointerTo(int(scanDurationSec)),
	}

	return &estimation, nil
}

func findMatchingStatsForInput(stats *[]models.AssetScanInputScanStats, inputType, inputPath string) (models.AssetScanInputScanStats, bool) {
	if stats == nil {
		return models.AssetScanInputScanStats{}, false
	}
	for i, scanStats := range *stats {
		if *scanStats.Type == inputType && *scanStats.Path == inputPath {
			ret := *stats
			return ret[i], true
		}
	}
	return models.AssetScanInputScanStats{}, false
}

func getScanSize(stats models.AssetScanStats, asset *models.Asset, input familiestypes.Input) (int64, error) {
	var scanSizeMB int64

	inputType := input.InputType
	inputPath := input.Input

	sbomStats, ok := findMatchingStatsForInput(stats.Sbom, inputType, inputPath)
	if ok {
		if sbomStats.Size != nil && *sbomStats.Size > 0 {
			return *sbomStats.Size, nil
		}
	}

	vulStats, ok := findMatchingStatsForInput(stats.Vulnerabilities, inputType, inputPath)
	if ok {
		if vulStats.Size != nil && *vulStats.Size > 0 {
			return *vulStats.Size, nil
		}
	}

	secretsStats, ok := findMatchingStatsForInput(stats.Secrets, inputType, inputPath)
	if ok {
		if secretsStats.Size != nil && *secretsStats.Size > 0 {
			return *secretsStats.Size, nil
		}
	}

	exploitsStats, ok := findMatchingStatsForInput(stats.Exploits, inputType, inputPath)
	if ok {
		if exploitsStats.Size != nil && *exploitsStats.Size > 0 {
			return *exploitsStats.Size, nil
		}
	}

	rootkitsStats, ok := findMatchingStatsForInput(stats.Rootkits, inputType, inputPath)
	if ok {
		if rootkitsStats.Size != nil && *rootkitsStats.Size > 0 {
			return *rootkitsStats.Size, nil
		}
	}

	misconfigurationsStats, ok := findMatchingStatsForInput(stats.Misconfigurations, inputType, inputPath)
	if ok {
		if misconfigurationsStats.Size != nil && *misconfigurationsStats.Size > 0 {
			return *misconfigurationsStats.Size, nil
		}
	}

	malwareStats, ok := findMatchingStatsForInput(stats.Malware, inputType, inputPath)
	if ok {
		if malwareStats.Size != nil && *malwareStats.Size > 0 {
			return *malwareStats.Size, nil
		}
	}

	// if no scan size was found from the previous scan stats, estimate the scan size from the asset root volume size
	vminfo, err := asset.AssetInfo.AsVMInfo()
	if err != nil {
		return 0, fmt.Errorf("failed to use asset info as vminfo: %v", err)
	}
	sourceVolumeSizeMB := int64(vminfo.RootVolume.SizeGB * 1000)
	scanSizeMB = sourceVolumeSizeMB / 2 // Volumes are normally only about 50% full

	return scanSizeMB, nil
}

// search in all the families stats and look for the first family (by random order) that has scan size stats for the specific input.
func getScanDuration(stats models.AssetScanStats, familiesConfig *models.ScanFamiliesConfig, scanSizeMB int64, input familiestypes.Input) (duration int64, err error) {
	var totalScanDuration int64

	scanSizeGB := float64(scanSizeMB) / 1000

	if familiesConfig.Sbom.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Sbom, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			// if we didn't find the duration from the stats, take it from our static scan duration map.
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.SBOM] * scanSizeGB)
		}
	}

	if familiesConfig.Vulnerabilities.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Vulnerabilities, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.Vulnerabilities] * scanSizeGB)
		}
	}

	if familiesConfig.Secrets.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Secrets, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.Secrets] * scanSizeGB)
		}
	}

	if familiesConfig.Exploits.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Exploits, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.Exploits] * scanSizeGB)
		}
	}

	if familiesConfig.Rootkits.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Rootkits, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.Rootkits] * scanSizeGB)
		}
	}

	if familiesConfig.Misconfigurations.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Misconfigurations, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.Misconfiguration] * scanSizeGB)
		}
	}

	if familiesConfig.Malware.IsEnabled() {
		scanDuration := getScanDurationFromStats(stats.Malware, input)
		if scanDuration != 0 {
			totalScanDuration += scanDuration
		} else {
			totalScanDuration += int64(familyScanDurationPerGBMap[familiestypes.Malware] * scanSizeGB)
		}
	}

	return totalScanDuration, nil
}

func getScanDurationFromStats(stats *[]models.AssetScanInputScanStats, input familiestypes.Input) int64 {
	stat, ok := findMatchingStatsForInput(stats, input.InputType, input.Input)
	if !ok {
		return 0
	}

	if stat.ScanTime == nil {
		return 0
	}
	if stat.ScanTime.EndTime == nil || stat.ScanTime.StartTime == nil {
		return 0
	}

	dur := stat.ScanTime.EndTime.Sub(*stat.ScanTime.StartTime)

	durSec := dur / (1000 * 1000 * 1000)

	return int64(durSec)
}
