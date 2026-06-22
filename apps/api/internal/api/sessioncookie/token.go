package sessioncookie

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

const (
	csrfTokenPrefix       = "vf_csrf_"
	csrfTokenEntropyBytes = 32
)

var (
	ErrCSRFTokenMalformed             = errors.New("CSRF token is malformed")
	ErrCSRFTokenGenerationUnavailable = errors.New("CSRF token generation is unavailable")

	csrfTokenEncodedLength = base64.RawURLEncoding.EncodedLen(csrfTokenEntropyBytes)
)

type CSRFToken struct {
	value string
}

type CSRFTokenGenerator struct {
	random io.Reader
}

func NewCSRFTokenGenerator() *CSRFTokenGenerator {
	return &CSRFTokenGenerator{random: rand.Reader}
}

func (generator *CSRFTokenGenerator) Generate(ctx context.Context) (CSRFToken, error) {
	if err := ctx.Err(); err != nil {
		return CSRFToken{}, err
	}

	if generator == nil || generator.random == nil {
		return CSRFToken{}, fmt.Errorf("generate CSRF token: %w", ErrCSRFTokenGenerationUnavailable)
	}

	randomBytes := make([]byte, csrfTokenEntropyBytes)

	if _, err := io.ReadFull(generator.random, randomBytes); err != nil {
		return CSRFToken{}, fmt.Errorf("generate CSRF token: %w", ErrCSRFTokenGenerationUnavailable)
	}

	if err := ctx.Err(); err != nil {
		return CSRFToken{}, err
	}

	encodedValue := base64.RawURLEncoding.EncodeToString(randomBytes)

	return CSRFToken{value: csrfTokenPrefix + encodedValue}, nil
}

func ParseCSRFToken(value string) (CSRFToken, error) {
	if len(value) != len(csrfTokenPrefix)+csrfTokenEncodedLength ||
		!strings.HasPrefix(value, csrfTokenPrefix) {
		return CSRFToken{}, ErrCSRFTokenMalformed
	}

	encodedValue := strings.TrimPrefix(value, csrfTokenPrefix)

	decodedValue, err := base64.RawURLEncoding.DecodeString(encodedValue)
	if err != nil ||
		len(decodedValue) != csrfTokenEntropyBytes ||
		base64.RawURLEncoding.EncodeToString(decodedValue) != encodedValue {
		return CSRFToken{}, ErrCSRFTokenMalformed
	}

	return CSRFToken{value: value}, nil
}

func (token CSRFToken) Value() string {
	return token.value
}

func (token CSRFToken) Equal(other CSRFToken) bool {
	if len(token.value) != len(other.value) {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(token.value), []byte(other.value)) == 1
}

func (token CSRFToken) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "[REDACTED]")
}
