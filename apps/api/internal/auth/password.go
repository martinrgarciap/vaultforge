package auth

import (
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const (
	MinPasswordLength = 15
	MaxPasswordLength = 128
)

// NormalizeAndValidatePassword converts canonically equivalent Unicode
// representations to NFC and validates the resulting password.
//
// It does not trim spaces, change capitalization, remove characters,
// or silently truncate the password.
func NormalizeAndValidatePassword(
	password string,
) (string, error) {
	if !utf8.ValidString(password) {
		return "", ErrPasswordInvalidUTF8
	}

	normalizedPassword := norm.NFC.String(password)
	passwordLength := utf8.RuneCountInString(
		normalizedPassword,
	)

	switch {
	case passwordLength < MinPasswordLength:
		return "", ErrPasswordTooShort

	case passwordLength > MaxPasswordLength:
		return "", ErrPasswordTooLong

	default:
		return normalizedPassword, nil
	}
}

// ValidatePassword checks whether a password satisfies the account
// password policy.
func ValidatePassword(password string) error {
	_, err := NormalizeAndValidatePassword(password)

	return err
}
