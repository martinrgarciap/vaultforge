package auth

import (
	"net/mail"
	"strings"
)

const MaxEmailLength = 254

func NormalizeEmail(
	email string,
) (string, error) {
	normalizedEmail := strings.ToLower(
		strings.TrimSpace(email),
	)

	if normalizedEmail == "" ||
		len(normalizedEmail) > MaxEmailLength {
		return "", ErrEmailInvalid
	}

	if strings.ContainsAny(
		normalizedEmail,
		"\r\n",
	) {
		return "", ErrEmailInvalid
	}

	parsedAddress, err := mail.ParseAddress(
		normalizedEmail,
	)
	if err != nil ||
		parsedAddress.Name != "" ||
		parsedAddress.Address != normalizedEmail {
		return "", ErrEmailInvalid
	}

	localPart, domain, found := strings.Cut(
		normalizedEmail,
		"@",
	)
	if !found ||
		localPart == "" ||
		domain == "" {
		return "", ErrEmailInvalid
	}

	return normalizedEmail, nil
}
