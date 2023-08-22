package scanestimation

import (
	"context"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type FakeOfferFileFetcher struct{}

func (f *FakeOfferFileFetcher) GetSnapshotMonthlyCostPerGB(ctx context.Context, regionCode string) (float64, error) {
	return 0, nil
}
func (f *FakeOfferFileFetcher) GetVolumeMonthlyCostPerGB(ctx context.Context, regionCode string, volumeType ec2types.VolumeType) (float64, error) {
	return 0, nil
}
func (f *FakeOfferFileFetcher) GetDataTransferCostPerGB(sourceRegion, destRegion string) (float64, error) {
	return 0, nil
}
func (f *FakeOfferFileFetcher) GetInstancePerHourCost(ctx context.Context, regionCode string, instanceType ec2types.InstanceType) (float64, error) {
	return 0, nil
}
