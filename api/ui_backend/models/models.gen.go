// Package models provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.12.3 DO NOT EDIT.
package models

// ApiResponse An object that is returned in all cases of failures.
type ApiResponse struct {
	Message *string `json:"message,omitempty"`
}

// RiskiestRegions Riskiest regions.
type RiskiestRegions struct {
	Message *string `json:"message,omitempty"`
}

// ExampleFilter defines model for exampleFilter.
type ExampleFilter = string

// UnknownError An object that is returned in all cases of failures.
type UnknownError = ApiResponse

// GetDashboardRiskiestRegionsParams defines parameters for GetDashboardRiskiestRegions.
type GetDashboardRiskiestRegionsParams struct {
	Example *ExampleFilter `form:"example,omitempty" json:"example,omitempty"`
}
