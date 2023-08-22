package scanestimation

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/pricing"
	"github.com/aws/aws-sdk-go-v2/service/pricing/types"

	"github.com/openclarity/vmclarity/pkg/shared/utils"
)

type OfferFileFetcher interface {
	GetSnapshotMonthlyCostPerGB(ctx context.Context, regionCode string) (float64, error)
	GetVolumeMonthlyCostPerGB(ctx context.Context, regionCode string, volumeType ec2types.VolumeType) (float64, error)
	GetDataTransferCostPerGB(sourceRegion, destRegion string) (float64, error)
	GetInstancePerHourCost(ctx context.Context, regionCode string, instanceType ec2types.InstanceType) (float64, error)
}

type OfferFileFetcherImpl struct {
	pricingClient *pricing.Client
}

func (o *OfferFileFetcherImpl) GetSnapshotMonthlyCostPerGB(ctx context.Context, regionCode string) (float64, error) {
	usageType, ok := regionCodeToSnapshotUsageType[regionCode]
	if !ok {
		return 0, fmt.Errorf("failed to find usage type in regionCodeToSnapshotUsageType map with regionCode %v", regionCode)
	}

	return o.getPricePerUnit(ctx, usageType, "")
}
func (o *OfferFileFetcherImpl) GetVolumeMonthlyCostPerGB(ctx context.Context, regionCode string, volumeType ec2types.VolumeType) (float64, error) {
	usageType, ok := regionCodeToVolumeUsageType[regionCode]
	if !ok {
		return 0, fmt.Errorf("failed to find usage type in regionCodeToVolumeUsageType map with regionCode %v", regionCode)
	}
	usageType = fmt.Sprintf("%v.%v", usageType, volumeType)

	return o.getPricePerUnit(ctx, usageType, "")
}

func (o *OfferFileFetcherImpl) GetDataTransferCostPerGB(sourceRegion, destRegion string) (float64, error) {
	// TODO currently was not finding a reliable way to get the data transfer cost from the offer file.
	// This is the general price according to https://aws.amazon.com/ec2/pricing/on-demand/
	if destRegion == sourceRegion {
		return 0, nil
	}
	return 0.02, nil
}

func (o *OfferFileFetcherImpl) GetInstancePerHourCost(ctx context.Context, regionCode string, instanceType ec2types.InstanceType) (float64, error) {
	usageType, ok := regionCodeToInstanceUsageType[regionCode]
	if !ok {
		return 0, fmt.Errorf("failed to find usage type in regionCodeToInstanceUsageType map with regionCode %v", regionCode)
	}
	usageType = fmt.Sprintf("%v:%v", usageType, instanceType)

	return o.getPricePerUnit(ctx, usageType, "RunInstances")
}

func (o *OfferFileFetcherImpl) getPricePerUnit(ctx context.Context, usageType, operation string) (float64, error) {
	filters := createGetProductsFilters(usageType, "AmazonEC2", operation)

	products, err := o.pricingClient.GetProducts(ctx, &pricing.GetProductsInput{
		ServiceCode:   utils.PointerTo("AmazonEC2"),
		Filters:       filters,
		FormatVersion: utils.PointerTo("aws_v1"),
	}, func(options *pricing.Options) {
		options.Region = "us-east-1"
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get products. usageType=%v: %v", usageType, err)
	}
	if len(products.PriceList) != 1 {
		return 0, fmt.Errorf("got more than one product")
	}

	priceStr, err := getPricePerUnitFromJsonPriceList(products.PriceList[0])
	if err != nil {
		return 0, fmt.Errorf("failed to get pricePerUnit from json price list: %v", err)
	}

	return strconv.ParseFloat(priceStr, 64)
}

func createGetProductsFilters(usageType, serviceCode, operation string) []types.Filter {
	filters := []types.Filter{
		// An AWS SKU uniquely combines product (service code), Usage Type, and Operation for an AWS resource.
		// See https://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/procedures.html for more details
		{
			Field: utils.PointerTo("ServiceCode"),
			Type:  "TERM_MATCH",
			Value: &serviceCode,
		},
		{
			Field: utils.PointerTo("usagetype"),
			Type:  "TERM_MATCH",
			Value: &usageType,
		},
	}
	if operation != "" {
		filters = append(filters, types.Filter{
			Field: utils.PointerTo("operation"),
			Type:  "TERM_MATCH",
			Value: &operation,
		})
	}

	return filters
}

// parse the price list which exists in json format, in order to get the pricePerUnit property
func getPricePerUnitFromJsonPriceList(jsonPriceList string) (string, error) {
	var productMap map[string]any
	err := json.Unmarshal([]byte(jsonPriceList), &productMap)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal. jsonPriceList=%s:  %v", jsonPriceList, err)
	}
	termsMap, ok := productMap["terms"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("terms key was not found in map. productMap=%v", productMap)
	}
	ondemandMap, ok := termsMap["OnDemand"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("OnDemand key was not found in map. termsMap=%v", termsMap)
	}

	// since we don't know what the key name is, we will assume this is a map with size of 1
	// The key here is the rate code. I can do a map of rate codes per region, but I am not sure how persistent the rate codes are, and if they can change when there is a new rate.
	val, err := getFirstValueFromMap(ondemandMap)
	if err != nil {
		return "", fmt.Errorf("failed to get first value from map. ondemandMap=%v", ondemandMap)
	}
	priceDimensionsMap, ok := val["priceDimensions"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("priceDimensions key was not found in map. map=%v", val)
	}
	val, err = getFirstValueFromMap(priceDimensionsMap)
	if err != nil {
		return "", fmt.Errorf("failed to get first value from map. priceDimensionsMap=%v", priceDimensionsMap)
	}
	pricePerUnitMap, ok := val["pricePerUnit"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("pricePerUnit key was not found in map. map=%v", val)
	}

	pricePerUnit, ok := pricePerUnitMap["USD"].(string)
	if !ok {
		return "", fmt.Errorf("USD key was not found in map. pricePerUnitMap=%v", pricePerUnitMap)
	}

	return pricePerUnit, nil
}

func getFirstValueFromMap(m map[string]any) (map[string]any, error) {
	for _, val := range m {
		ret, ok := val.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("failed to convert value into map[string]any. value=%v", val)
		}

		return ret, nil
	}
	return nil, fmt.Errorf("map is empty")
}
