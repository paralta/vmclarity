package scanestimation

import (
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/pricing/types"

	"github.com/openclarity/vmclarity/pkg/shared/utils"
)

func Test_createGetProductsFilters(t *testing.T) {
	type args struct {
		usageType   string
		serviceCode string
		operation   string
	}
	tests := []struct {
		name string
		args args
		want []types.Filter
	}{
		{
			name: "no operation",
			args: args{
				usageType:   "sampleUsageType",
				serviceCode: "sampleServiceCode",
				operation:   "",
			},
			want: []types.Filter{
				{
					Field: utils.PointerTo("ServiceCode"),
					Type:  "TERM_MATCH",
					Value: utils.PointerTo("sampleServiceCode"),
				},
				{
					Field: utils.PointerTo("usagetype"),
					Type:  "TERM_MATCH",
					Value: utils.PointerTo("sampleUsageType"),
				},
			},
		},
		{
			name: "with operation",
			args: args{
				usageType:   "sampleUsageType",
				serviceCode: "sampleServiceCode",
				operation:   "sampleOperation",
			},
			want: []types.Filter{
				{
					Field: utils.PointerTo("ServiceCode"),
					Type:  "TERM_MATCH",
					Value: utils.PointerTo("sampleServiceCode"),
				},
				{
					Field: utils.PointerTo("usagetype"),
					Type:  "TERM_MATCH",
					Value: utils.PointerTo("sampleUsageType"),
				},
				{
					Field: utils.PointerTo("operation"),
					Type:  "TERM_MATCH",
					Value: utils.PointerTo("sampleOperation"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := createGetProductsFilters(tt.args.usageType, tt.args.serviceCode, tt.args.operation); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createGetProductsFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getPricePerUnitFromJsonPriceList(t *testing.T) {
	type args struct {
		jsonPriceList string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getPricePerUnitFromJsonPriceList(tt.args.jsonPriceList)
			if (err != nil) != tt.wantErr {
				t.Errorf("getPricePerUnitFromJsonPriceList() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("getPricePerUnitFromJsonPriceList() got = %v, want %v", got, tt.want)
			}
		})
	}
}
