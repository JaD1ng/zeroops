package model

import (
	"shared/faults"
)

var (
	ErrMetadataNotFound = faults.NewError(
		faults.ErrorTypeObjectNotFound,
		"METADATA_NOT_FOUND",
		"The specified metadata entry does not exist",
	)

	ErrMetadataAlreadyExists = faults.NewError(
		faults.ErrorTypeConflict,
		"METADATA_ALREADY_EXISTS",
		"The metadata entry already exists",
	)

	ErrInvalidKey = faults.NewError(
		faults.ErrorTypeBadRequest,
		"INVALID_KEY",
		"The specified key is not valid",
	)

	ErrInvalidMetadata = faults.NewError(
		faults.ErrorTypeBadRequest,
		"INVALID_METADATA",
		"The metadata is invalid",
	)
)
