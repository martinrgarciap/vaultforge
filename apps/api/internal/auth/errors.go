package auth

import "errors"

var (
	ErrEmailInvalid = errors.New(
		"email is invalid",
	)
	ErrEmailUnavailable = errors.New(
		"email is unavailable",
	)
	ErrInvalidCredentials = errors.New(
		"invalid email or password",
	)
	ErrAuthenticationUnavailable = errors.New(
		"authentication is unavailable",
	)

	ErrPasswordInvalidUTF8 = errors.New(
		"password must contain valid UTF-8",
	)
	ErrPasswordTooShort = errors.New(
		"password is too short",
	)
	ErrPasswordTooLong = errors.New(
		"password is too long",
	)

	ErrPasswordMismatch = errors.New(
		"password does not match",
	)
	ErrPasswordHashMalformed = errors.New(
		"password hash is malformed",
	)
	ErrPasswordAlgorithmUnsupported = errors.New(
		"password algorithm is unsupported",
	)
	ErrPasswordHasherUnavailable = errors.New(
		"password hasher is unavailable",
	)
)
