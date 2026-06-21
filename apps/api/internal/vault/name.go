package vault

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

const MaxVaultNameLength = 128

func NormalizeName(
	name string,
) (string, error) {
	if !utf8.ValidString(name) {
		return "", ErrVaultNameInvalidUTF8
	}

	normalizedName := norm.NFC.String(name)

	for _, character := range normalizedName {
		if unicode.IsControl(character) {
			return "",
				ErrVaultNameContainsControlCharacter
		}
	}

	normalizedName = strings.TrimSpace(
		normalizedName,
	)

	if normalizedName == "" {
		return "", ErrVaultNameEmpty
	}

	if utf8.RuneCountInString(
		normalizedName,
	) > MaxVaultNameLength {
		return "", ErrVaultNameTooLong
	}

	return normalizedName, nil
}
