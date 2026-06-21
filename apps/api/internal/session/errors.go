package session

import "errors"

var (
	ErrRefreshTokenMalformed = errors.New(
		"refresh token is malformed",
	)

	ErrRefreshTokenInvalid = errors.New(
		"refresh token is invalid",
	)

	ErrRefreshTokenGenerationUnavailable = errors.New(
		"refresh token generation is unavailable",
	)

	ErrAccessTokenTTLInvalid = errors.New(
		"access token lifetime is invalid",
	)

	ErrRefreshTokenTTLInvalid = errors.New(
		"refresh token lifetime is invalid",
	)

	ErrClockLeewayInvalid = errors.New(
		"token clock leeway is invalid",
	)

	ErrAccessTokenConfigurationInvalid = errors.New(
		"access token configuration is invalid",
	)

	ErrAccessTokenInvalid = errors.New(
		"access token is invalid",
	)

	ErrAccessTokenUnavailable = errors.New(
		"access token service is unavailable",
	)

	ErrPrincipalInvalid = errors.New(
		"authenticated principal is invalid",
	)

	ErrSessionUnavailable = errors.New(
		"session service is unavailable",
	)

	ErrSessionNotFound = errors.New(
		"session was not found",
	)
)
