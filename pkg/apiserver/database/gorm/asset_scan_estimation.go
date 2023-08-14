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

package gorm

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/openclarity/vmclarity/api/models"
	"github.com/openclarity/vmclarity/pkg/apiserver/common"
	"github.com/openclarity/vmclarity/pkg/apiserver/database/types"
	"github.com/openclarity/vmclarity/pkg/shared/utils"
)

const (
	assetScanEstimationsSchemaName = "AssetScanEstimation"
)

type AssetScanEstimation struct {
	ODataObject
}

type AssetScanEstimationsTableHandler struct {
	DB *gorm.DB
}

func (db *Handler) AssetScanEstimationsTable() types.AssetScanEstimationsTable {
	return &AssetScanEstimationsTableHandler{
		DB: db.DB,
	}
}

func (s *AssetScanEstimationsTableHandler) GetAssetScanEstimations(params models.GetAssetScanEstimationsParams) (models.AssetScanEstimations, error) {
	var assetScanEstimations []AssetScanEstimation
	err := ODataQuery(s.DB, assetScanEstimationsSchemaName, params.Filter, params.Select, params.Expand, params.OrderBy, params.Top, params.Skip, true, &assetScanEstimations)
	if err != nil {
		return models.AssetScanEstimations{}, err
	}

	items := make([]models.AssetScanEstimation, len(assetScanEstimations))
	for i, assetScanEstimation := range assetScanEstimations {
		var as models.AssetScanEstimation
		if err = json.Unmarshal(assetScanEstimation.Data, &as); err != nil {
			return models.AssetScanEstimations{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
		}
		items[i] = as
	}

	output := models.AssetScanEstimations{Items: &items}

	if params.Count != nil && *params.Count {
		count, err := ODataCount(s.DB, assetScanEstimationsSchemaName, params.Filter)
		if err != nil {
			return models.AssetScanEstimations{}, fmt.Errorf("failed to count records: %w", err)
		}
		output.Count = &count
	}

	return output, nil
}

func (s *AssetScanEstimationsTableHandler) GetAssetScanEstimation(assetScanEstimationID models.AssetScanEstimationID, params models.GetAssetScanEstimationsAssetScanEstimationIDParams) (models.AssetScanEstimation, error) {
	var dbAssetScanEstimation AssetScanEstimation
	filter := fmt.Sprintf("id eq '%s'", assetScanEstimationID)
	err := ODataQuery(s.DB, assetScanEstimationsSchemaName, &filter, params.Select, params.Expand, nil, nil, nil, false, &dbAssetScanEstimation)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.AssetScanEstimation{}, types.ErrNotFound
		}
		return models.AssetScanEstimation{}, err
	}

	var as models.AssetScanEstimation
	err = json.Unmarshal(dbAssetScanEstimation.Data, &as)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	return as, nil
}

// nolint:cyclop
func (s *AssetScanEstimationsTableHandler) CreateAssetScanEstimation(assetScanEstimation models.AssetScanEstimation) (models.AssetScanEstimation, error) {
	// Check the user provided asset id field
	if assetScanEstimation.Asset != nil && assetScanEstimation.Asset.Id == "" {
		return models.AssetScanEstimation{}, &common.BadRequestError{
			Reason: "asset.id is a required field",
		}
	}

	// Check the user didn't provide an ID
	if assetScanEstimation.Id != nil {
		return models.AssetScanEstimation{}, &common.BadRequestError{
			Reason: "can not specify id field when creating a new AssetScanEstimation",
		}
	}

	// Generate a new UUID
	assetScanEstimation.Id = utils.PointerTo(uuid.New().String())

	// Initialise revision
	assetScanEstimation.Revision = utils.PointerTo(1)

	// TODO(sambetts) Lock the table here to prevent race conditions
	// checking the uniqueness.
	//
	// We might also be able to do this without locking the table by doing
	// a single query which includes the uniqueness check like:
	//
	// INSERT INTO scan_configs(data) SELECT * FROM (SELECT "<encoded json>") AS tmp WHERE NOT EXISTS (SELECT * FROM scan_configs WHERE JSON_EXTRACT(`Data`, '$.Name') = '<name from input>') LIMIT 1;
	//
	// This should return 0 affected fields if there is a conflicting
	// record in the DB, and should be treated safely by the DB without
	// locking the table.

	// Check the existing DB entries to ensure that the scan id and asset id fields are unique
	existingAssetScanEstimation, err := s.checkUniqueness(assetScanEstimation)
	if err != nil {
		var conflictErr *common.ConflictError
		if errors.As(err, &conflictErr) {
			return existingAssetScanEstimation, err
		}
		return models.AssetScanEstimation{}, fmt.Errorf("failed to check existing scan: %w", err)
	}

	marshaled, err := json.Marshal(assetScanEstimation)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert API model to DB model: %w", err)
	}

	newAssetScanEstimation := AssetScanEstimation{}
	newAssetScanEstimation.Data = marshaled

	if err := s.DB.Create(&newAssetScanEstimation).Error; err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to create asset scan Estimation in db: %w", err)
	}

	// TODO(sambetts) Maybe this isn't required now because the DB isn't
	// creating any of the data (like the ID) so we can just return the
	// assetScan pre-marshal above.
	var as models.AssetScanEstimation
	err = json.Unmarshal(newAssetScanEstimation.Data, &as)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	return as, nil
}

// nolint:cyclop,gocognit
func (s *AssetScanEstimationsTableHandler) SaveAssetScanEstimation(assetScanEstimation models.AssetScanEstimation, params models.PutAssetScanEstimationsAssetScanEstimationIDParams) (models.AssetScanEstimation, error) {
	if assetScanEstimation.Id == nil || *assetScanEstimation.Id == "" {
		return models.AssetScanEstimation{}, &common.BadRequestError{
			Reason: "id is required to save asset scan Estimation",
		}
	}

	// Check the user provided asset id field
	if assetScanEstimation.Asset != nil && assetScanEstimation.Asset.Id == "" {
		return models.AssetScanEstimation{}, &common.BadRequestError{
			Reason: "asset.id is a required field",
		}
	}

	// Check the existing DB entries to ensure that the scan id and asset id fields are unique
	existingAssetScanEstimation, err := s.checkUniqueness(assetScanEstimation)
	if err != nil {
		var conflictErr *common.ConflictError
		if errors.As(err, &conflictErr) {
			return existingAssetScanEstimation, err
		}
		return models.AssetScanEstimation{}, fmt.Errorf("failed to check existing scan: %w", err)
	}

	var dbObj AssetScanEstimation
	if err := getExistingObjByID(s.DB, assetScanEstimationsSchemaName, *assetScanEstimation.Id, &dbObj); err != nil {
		return models.AssetScanEstimation{}, err
	}

	var dbAssetScanEstimation models.AssetScanEstimation
	err = json.Unmarshal(dbObj.Data, &dbAssetScanEstimation)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	if err := checkRevisionEtag(params.IfMatch, dbAssetScanEstimation.Revision); err != nil {
		return models.AssetScanEstimation{}, err
	}

	assetScanEstimation.Revision = bumpRevision(dbAssetScanEstimation.Revision)

	marshaled, err := json.Marshal(assetScanEstimation)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert API model to DB model: %w", err)
	}

	dbObj.Data = marshaled

	if err := s.DB.Save(&dbObj).Error; err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to save asset scan Estimation in db: %w", err)
	}

	// TODO(sambetts) Maybe this isn't required now because the DB isn't
	// creating any of the data (like the ID) so we can just return the
	// assetScan pre-marshal above.
	var as models.AssetScanEstimation
	err = json.Unmarshal(dbObj.Data, &as)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	return as, nil
}

// nolint:cyclop
func (s *AssetScanEstimationsTableHandler) UpdateAssetScanEstimation(assetScanEstimation models.AssetScanEstimation, params models.PatchAssetScanEstimationsAssetScanEstimationIDParams) (models.AssetScanEstimation, error) {
	if assetScanEstimation.Id == nil || *assetScanEstimation.Id == "" {
		return models.AssetScanEstimation{}, &common.BadRequestError{
			Reason: "id is required to update asset scan Estimation",
		}
	}

	var dbObj AssetScanEstimation
	if err := getExistingObjByID(s.DB, assetScanEstimationsSchemaName, *assetScanEstimation.Id, &dbObj); err != nil {
		return models.AssetScanEstimation{}, err
	}

	var err error
	var dbAssetScanEstimation models.AssetScanEstimation
	err = json.Unmarshal(dbObj.Data, &dbAssetScanEstimation)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	if err := checkRevisionEtag(params.IfMatch, dbAssetScanEstimation.Revision); err != nil {
		return models.AssetScanEstimation{}, err
	}

	assetScanEstimation.Revision = bumpRevision(dbAssetScanEstimation.Revision)

	dbObj.Data, err = patchObject(dbObj.Data, assetScanEstimation)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to apply patch: %w", err)
	}

	var ase models.AssetScanEstimation
	err = json.Unmarshal(dbObj.Data, &ase)
	if err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	// Check the existing DB entries to ensure that the scan id and asset id fields are unique
	existingAssetScanEstimation, err := s.checkUniqueness(ase)
	if err != nil {
		var conflictErr *common.ConflictError
		if errors.As(err, &conflictErr) {
			return existingAssetScanEstimation, err
		}
		return models.AssetScanEstimation{}, fmt.Errorf("failed to check existing scan: %w", err)
	}

	if err := s.DB.Save(&dbObj).Error; err != nil {
		return models.AssetScanEstimation{}, fmt.Errorf("failed to save asset scan Estimation in db: %w", err)
	}

	return ase, nil
}

func (s *AssetScanEstimationsTableHandler) checkUniqueness(assetScan models.AssetScanEstimation) (models.AssetScanEstimation, error) {
	// We only check unique if scan is set, so return early if it's not set.
	if assetScan.Scan == nil || assetScan.Scan.Id == "" {
		return models.AssetScanEstimation{}, nil
	}

	// If Scan is set we need to check if there is another asset scan with
	// the same scan id and asset id.
	var assetScanEstimations []AssetScanEstimation
	filter := fmt.Sprintf("id ne '%s' and asset/id eq '%s' and scan/id eq '%s'", *assetScan.Id, assetScan.Asset.Id, assetScan.Scan.Id)
	err := ODataQuery(s.DB, assetScanEstimationsSchemaName, &filter, nil, nil, nil, nil, nil, true, &assetScanEstimations)
	if err != nil {
		return models.AssetScanEstimation{}, err
	}

	if len(assetScanEstimations) > 0 {
		var ase models.AssetScanEstimation
		if err = json.Unmarshal(assetScanEstimations[0].Data, &ase); err != nil {
			return models.AssetScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
		}
		return ase, &common.ConflictError{
			Reason: fmt.Sprintf("AssetScan exists with same asset id=%s and scan id=%s)", assetScan.Asset.Id, assetScan.Scan.Id),
		}
	}
	return models.AssetScanEstimation{}, nil
}
