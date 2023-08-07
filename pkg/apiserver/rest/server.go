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

package rest

import (
	"context"
	"fmt"
	"github.com/openclarity/vmclarity/pkg/apiserver/iam"
	"github.com/openclarity/vmclarity/pkg/apiserver/iam/provider"
	"time"

	"github.com/getkin/kin-openapi/openapi3filter"

	"github.com/deepmap/oapi-codegen/pkg/middleware"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/openclarity/vmclarity/api/server"
	"github.com/openclarity/vmclarity/pkg/apiserver/common"
	"github.com/openclarity/vmclarity/pkg/apiserver/config"
	databaseTypes "github.com/openclarity/vmclarity/pkg/apiserver/database/types"
	"github.com/openclarity/vmclarity/pkg/shared/log"
)

const (
	shutdownTimeoutSec = 10
)

type ServerImpl struct {
	dbHandler databaseTypes.Database
}

type Server struct {
	port       int
	echoServer *echo.Echo
}

func CreateRESTServer(config *config.Config, dbHandler databaseTypes.Database) (*Server, error) {
	e, err := createEchoServer(config, dbHandler)
	if err != nil {
		return nil, fmt.Errorf("failed to create rest server: %w", err)
	}
	return &Server{
		port:       config.BackendRestPort,
		echoServer: e,
	}, nil
}

func createEchoServer(config *config.Config, dbHandler databaseTypes.Database) (*echo.Echo, error) {
	swagger, err := server.GetSwagger()
	if err != nil {
		return nil, fmt.Errorf("failed to load swagger spec: %w", err)
	}

	// Create server
	e := echo.New()

	// Log all requests
	e.Use(echomiddleware.Logger())

	// Recover any panics into HTTP 500
	e.Use(echomiddleware.Recover())

	// Use oapi-codegen validation middleware to validate the API group against the
	// OpenAPI schema along with IAM provider if configured.
	if config.IamEnabled {
		iamProvider, err := provider.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create IAM provider: %v", err)
		}

		e.Use(middleware.OapiRequestValidatorWithOptions(swagger, &middleware.Options{
			Options: openapi3filter.Options{
				AuthenticationFunc: iam.OapiFilterForProvider(iamProvider),
			},
		}))
	} else {
		e.Use(middleware.OapiRequestValidator(swagger))
	}

	// Register paths with the backend implementation
	server.RegisterHandlers(e, &ServerImpl{
		dbHandler: dbHandler,
	})

	return e, nil
}

func (s *Server) Start(ctx context.Context, errChan chan struct{}) {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	logger.Infof("Starting REST server")
	go func() {
		if err := s.echoServer.Start(fmt.Sprintf("0.0.0.0:%d", s.port)); err != nil {
			logger.Errorf("Failed to start REST server: %v", err)
			errChan <- common.Empty
		}
	}()
	logger.Infof("REST server is running")
}

func (s *Server) Stop(ctx context.Context) {
	logger := log.GetLoggerFromContextOrDiscard(ctx)

	logger.Infof("Stopping REST server")
	if s.echoServer != nil {
		ctx, cancel := context.WithTimeout(ctx, shutdownTimeoutSec*time.Second)
		defer cancel()
		if err := s.echoServer.Shutdown(ctx); err != nil {
			logger.Errorf("Failed to shutdown REST server: %v", err)
		}
	}
}
