/*
 * Copyright (c) 2026 tqtrungse@gmail.com. All rights reserved.
 * SPDX-License-Identifier: Apache-2.0
 */

package bootstrap

import (
	"context"
	"errors"
	"fmt"

	iamhttp "xchat/internal/iam/adapter/http"
	"xchat/internal/iam/adapter/http/route"
	"xchat/internal/iam/application/registerdevice"
	"xchat/internal/iam/core/rejection"
	"xchat/internal/iam/infrastructure/crypto"
	"xchat/internal/iam/infrastructure/mysql"

	"xchat/pkg/config"
	"xchat/pkg/database/sql"
	"xchat/pkg/generator"
	"xchat/pkg/log"
	"xchat/pkg/reject"
)

// buildRoutes connects the dependencies of each use case by hand and gives the
// HTTP handlers that serve them. It opens the database. closeFn closes the
// database. The caller calls closeFn after the server stops, and not before:
// a request that is still active uses the database.
//
// To add a use case, build its dependencies here, make its handler, and append
// the handler to the result.
func buildRoutes(
	ctx context.Context,
	loader config.Loader,
	logger *log.Logger,
	rejections *reject.Stats[rejection.Reason],
) (routes []iamhttp.Mounter, closeFn func() error, err error) {
	dbConfig, err := sql.LoadConfig(loader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load database config: %w", err)
	}

	id, err := machineID(loader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to make the machine ID: %w", err)
	}
	intGenerator, err := generator.NewInt(id)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create the ID generator: %w", err)
	}

	db := sql.NewMySql(logger)
	closeDB, err := db.Open(ctx, dbConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open the database: %w", err)
	}
	defer func() {
		// Do not leave the database open if a later step fails. A return that
		// gives an error also sets closeFn to nil, so the deferred call uses
		// closeDB.
		if err != nil {
			err = errors.Join(err, closeDB())
		}
	}()

	// The methods of these types have pointer receivers, so the pointers, and not
	// the values, implement the ports of the use cases.
	var (
		keyDeriver     = crypto.NewX25519()
		passwordHasher = crypto.NewBcrypt(0) // The default is 3/4 of GOMAXPROCS.
		userReader     = mysql.NewUserReader()
		userWriter     = mysql.NewUserWriter()
		deviceWriter   = mysql.NewDeviceWriter()
	)

	registerDeviceUsecase := registerdevice.New(
		db,
		logger,
		intGenerator,
		&keyDeriver,
		&passwordHasher,
		&userReader,
		&userWriter,
		&deviceWriter,
	)
	registerDeviceHandler, err := route.NewRegisterDeviceHandler(registerDeviceUsecase, logger, rejections)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create the register device handler: %w", err)
	}

	return []iamhttp.Mounter{registerDeviceHandler}, closeDB, nil
}
