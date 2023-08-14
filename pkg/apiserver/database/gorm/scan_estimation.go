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
	scanEstimationSchemaName = "ScanEstimation"
)

type ScanEstimation struct {
	ODataObject
}

type ScanEstimationsTableHandler struct {
	DB *gorm.DB
}

func (db *Handler) ScanEstimationsTable() types.ScanEstimationsTable {
	return &ScanEstimationsTableHandler{
		DB: db.DB,
	}
}

func (s *ScanEstimationsTableHandler) GetScanEstimations(params models.GetScanEstimationsParams) (models.ScanEstimations, error) {
	var scanEstimations []ScanEstimation
	err := ODataQuery(s.DB, scanEstimationSchemaName, params.Filter, params.Select, params.Expand, params.OrderBy, params.Top, params.Skip, true, &scanEstimations)
	if err != nil {
		return models.ScanEstimations{}, err
	}

	items := make([]models.ScanEstimation, len(scanEstimations))
	for i, sc := range scanEstimations {
		var scanEstimation models.ScanEstimation
		err = json.Unmarshal(sc.Data, &scanEstimation)
		if err != nil {
			return models.ScanEstimations{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
		}
		items[i] = scanEstimation
	}

	output := models.ScanEstimations{Items: &items}

	if params.Count != nil && *params.Count {
		count, err := ODataCount(s.DB, scanEstimationSchemaName, params.Filter)
		if err != nil {
			return models.ScanEstimations{}, fmt.Errorf("failed to count records: %w", err)
		}
		output.Count = &count
	}

	return output, nil
}

func (s *ScanEstimationsTableHandler) GetScanEstimation(scanEstimationID models.ScanEstimationID, params models.GetScanEstimationsScanEstimationIDParams) (models.ScanEstimation, error) {
	var dbScanEstimation ScanEstimation
	filter := fmt.Sprintf("id eq '%s'", scanEstimationID)
	err := ODataQuery(s.DB, scanEstimationSchemaName, &filter, params.Select, params.Expand, nil, nil, nil, false, &dbScanEstimation)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.ScanEstimation{}, types.ErrNotFound
		}
		return models.ScanEstimation{}, err
	}

	var apiScanEstimation models.ScanEstimation
	err = json.Unmarshal(dbScanEstimation.Data, &apiScanEstimation)
	if err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	return apiScanEstimation, nil
}

func (s *ScanEstimationsTableHandler) CreateScanEstimation(scanEstimation models.ScanEstimation) (models.ScanEstimation, error) {
	// Check the user didn't provide an ID
	if scanEstimation.Id != nil {
		return models.ScanEstimation{}, &common.BadRequestError{
			Reason: "can not specify id field when creating a new ScanEstimation",
		}
	}

	// Generate a new UUID
	scanEstimation.Id = utils.PointerTo(uuid.New().String())

	// Initialise revision
	scanEstimation.Revision = utils.PointerTo(1)

	// TODO do we want ScanConfig to be required in the api?
	if scanEstimation.ScanConfig != nil {
		existingScanEstimation, err := s.checkUniqueness(scanEstimation)
		if err != nil {
			var conflictErr *common.ConflictError
			if errors.As(err, &conflictErr) {
				return existingScanEstimation, err
			}
			return models.ScanEstimation{}, fmt.Errorf("failed to check existing scan Estimation: %w", err)
		}
	}

	marshaled, err := json.Marshal(scanEstimation)
	if err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert API model to DB model: %w", err)
	}

	newScanEstimation := ScanEstimation{}
	newScanEstimation.Data = marshaled

	if err = s.DB.Create(&newScanEstimation).Error; err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to create scan Estimation in db: %w", err)
	}

	var apiScanEstimation models.ScanEstimation
	err = json.Unmarshal(newScanEstimation.Data, &apiScanEstimation)
	if err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	return apiScanEstimation, nil
}

// nolint:cyclop
func (s *ScanEstimationsTableHandler) SaveScanEstimation(scanEstimation models.ScanEstimation, params models.PutScanEstimationsScanEstimationIDParams) (models.ScanEstimation, error) {
	if scanEstimation.Id == nil || *scanEstimation.Id == "" {
		return models.ScanEstimation{}, &common.BadRequestError{
			Reason: "id is required to save scan Estimation",
		}
	}

	var dbObj ScanEstimation
	if err := getExistingObjByID(s.DB, scanEstimationSchemaName, *scanEstimation.Id, &dbObj); err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to get scan Estimation from db: %w", err)
	}

	var dbScanEstimation models.ScanEstimation
	if err := json.Unmarshal(dbObj.Data, &dbScanEstimation); err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert DB object to API model: %w", err)
	}

	if err := checkRevisionEtag(params.IfMatch, dbScanEstimation.Revision); err != nil {
		return models.ScanEstimation{}, err
	}

	if err := validateScanConfigID(scanEstimation, dbScanEstimation); err != nil {
		var badRequestErr *common.BadRequestError
		if errors.As(err, &badRequestErr) {
			return models.ScanEstimation{}, err
		}
		return models.ScanEstimation{}, fmt.Errorf("scan config id validation failed: %w", err)
	}

	if scanEstimation.ScanConfig != nil {
		existingScanEstimation, err := s.checkUniqueness(scanEstimation)
		if err != nil {
			var conflictErr *common.ConflictError
			if errors.As(err, &conflictErr) {
				return existingScanEstimation, err
			}
			return models.ScanEstimation{}, fmt.Errorf("failed to check existing scan Estimation: %w", err)
		}
	}

	scanEstimation.Revision = bumpRevision(dbScanEstimation.Revision)

	marshaled, err := json.Marshal(scanEstimation)
	if err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert API model to DB model: %w", err)
	}

	dbObj.Data = marshaled

	if err = s.DB.Save(&dbObj).Error; err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to save scan Estimation in db: %w", err)
	}

	var apiScanEstimation models.ScanEstimation
	if err = json.Unmarshal(dbObj.Data, &apiScanEstimation); err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	return apiScanEstimation, nil
}

// nolint:cyclop
func (s *ScanEstimationsTableHandler) UpdateScanEstimation(scanEstimation models.ScanEstimation, params models.PatchScanEstimationsScanEstimationIDParams) (models.ScanEstimation, error) {
	if scanEstimation.Id == nil || *scanEstimation.Id == "" {
		return models.ScanEstimation{}, &common.BadRequestError{
			Reason: "id is required to update scan Estimation",
		}
	}

	var dbObj ScanEstimation
	if err := getExistingObjByID(s.DB, scanEstimationSchemaName, *scanEstimation.Id, &dbObj); err != nil {
		return models.ScanEstimation{}, err
	}

	var dbScanEstimation models.ScanEstimation
	if err := json.Unmarshal(dbObj.Data, &dbScanEstimation); err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert DB object to API model: %w", err)
	}

	if err := checkRevisionEtag(params.IfMatch, dbScanEstimation.Revision); err != nil {
		return models.ScanEstimation{}, err
	}

	if err := validateScanConfigID(scanEstimation, dbScanEstimation); err != nil {
		var badRequestErr *common.BadRequestError
		if errors.As(err, &badRequestErr) {
			return models.ScanEstimation{}, err
		}
		return models.ScanEstimation{}, fmt.Errorf("scan config id validation failed: %w", err)
	}

	scanEstimation.Revision = bumpRevision(dbScanEstimation.Revision)

	var err error
	dbObj.Data, err = patchObject(dbObj.Data, scanEstimation)
	if err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to apply patch: %w", err)
	}

	var ret models.ScanEstimation
	err = json.Unmarshal(dbObj.Data, &ret)
	if err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
	}

	if ret.ScanConfig != nil {
		existingScanEstimation, err := s.checkUniqueness(ret)
		if err != nil {
			var conflictErr *common.ConflictError
			if errors.As(err, &conflictErr) {
				return existingScanEstimation, err
			}
			return models.ScanEstimation{}, fmt.Errorf("failed to check existing scan Estimation: %w", err)
		}
	}

	if err := s.DB.Save(&dbObj).Error; err != nil {
		return models.ScanEstimation{}, fmt.Errorf("failed to save scan Estimation in db: %w", err)
	}

	return ret, nil
}

func (s *ScanEstimationsTableHandler) DeleteScanEstimation(scanEstimationID models.ScanEstimationID) error {
	if err := deleteObjByID(s.DB, scanEstimationID, &ScanEstimation{}); err != nil {
		return fmt.Errorf("failed to delete scan Estimation: %w", err)
	}

	return nil
}

func (s *ScanEstimationsTableHandler) checkUniqueness(scanEstimation models.ScanEstimation) (models.ScanEstimation, error) {
	var scanEstimations []ScanEstimation
	// In the case of creating or updating a scan, needs to be checked whether other running scan exists with same scan config id.
	filter := fmt.Sprintf("id ne '%s' and scanConfig/id eq '%s' and endTime eq null", *scanEstimation.Id, scanEstimation.ScanConfig.Id)
	err := ODataQuery(s.DB, scanEstimationSchemaName, &filter, nil, nil, nil, nil, nil, true, &scanEstimations)
	if err != nil {
		return models.ScanEstimation{}, err
	}

	if len(scanEstimations) > 0 {
		var apiScanEstimation models.ScanEstimation
		if err := json.Unmarshal(scanEstimations[0].Data, &apiScanEstimation); err != nil {
			return models.ScanEstimation{}, fmt.Errorf("failed to convert DB model to API model: %w", err)
		}
		// If the scan that we want to modify is already finished it can be changed.
		// In the case of creating a new scan the end time will be nil.
		if scanEstimation.EndTime == nil {
			return apiScanEstimation, &common.ConflictError{
				Reason: fmt.Sprintf("Runnig scan Estimation exists with same scanConfigID=%q", scanEstimation.ScanConfig.Id),
			}
		}
	}
	return models.ScanEstimation{}, nil
}
