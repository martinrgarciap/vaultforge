package sessioncookie

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCSRFTokenGeneratorGenerate(t *testing.T) {
	t.Parallel()

	randomBytes := bytes.Repeat([]byte{0xab}, csrfTokenEntropyBytes)
	generator := &CSRFTokenGenerator{random: bytes.NewReader(randomBytes)}

	token, err := generator.Generate(context.Background())
	if err != nil {
		t.Fatalf("generate CSRF token: %v", err)
	}

	expectedValue := csrfTokenPrefix + base64.RawURLEncoding.EncodeToString(randomBytes)
	if token.Value() != expectedValue {
		t.Fatal("generated CSRF token did not match the deterministic test value")
	}

	parsedToken, err := ParseCSRFToken(token.Value())
	if err != nil {
		t.Fatalf("parse generated CSRF token: %v", err)
	}

	if parsedToken.Value() != token.Value() {
		t.Fatal("parsed CSRF token did not preserve its value")
	}
}

func TestCSRFTokenGeneratorCreatesUniqueTokens(t *testing.T) {
	t.Parallel()

	generator := NewCSRFTokenGenerator()
	seen := make(map[string]struct{}, 100)

	for range 100 {
		token, err := generator.Generate(context.Background())
		if err != nil {
			t.Fatalf("generate CSRF token: %v", err)
		}

		if _, exists := seen[token.Value()]; exists {
			t.Fatal("generated duplicate CSRF token")
		}

		seen[token.Value()] = struct{}{}
	}
}

func TestCSRFTokenGeneratorHonorsCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	generator := NewCSRFTokenGenerator()

	_, err := generator.Generate(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Generate() error = %v, want context.Canceled", err)
	}
}

func TestCSRFTokenGeneratorRejectsUnavailableRandomness(t *testing.T) {
	t.Parallel()

	generator := &CSRFTokenGenerator{
		random: csrfErrorReader{err: errors.New("random source failed")},
	}

	_, err := generator.Generate(context.Background())
	if !errors.Is(err, ErrCSRFTokenGenerationUnavailable) {
		t.Fatalf("Generate() error = %v, want ErrCSRFTokenGenerationUnavailable", err)
	}
}

func TestParseCSRFTokenRejectsMalformedValues(t *testing.T) {
	t.Parallel()

	validEncodedValue := base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xcd}, csrfTokenEntropyBytes))

	testCases := map[string]string{
		"empty":            "",
		"missing prefix":   validEncodedValue,
		"wrong prefix":     "wrong_" + validEncodedValue,
		"too short":        csrfTokenPrefix + validEncodedValue[:len(validEncodedValue)-1],
		"too long":         csrfTokenPrefix + validEncodedValue + "a",
		"invalid encoding": csrfTokenPrefix + strings.Repeat("*", csrfTokenEncodedLength),
	}

	for name, value := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := ParseCSRFToken(value)
			if !errors.Is(err, ErrCSRFTokenMalformed) {
				t.Fatalf("ParseCSRFToken() error = %v, want ErrCSRFTokenMalformed", err)
			}
		})
	}
}

func TestCSRFTokenEqual(t *testing.T) {
	t.Parallel()

	firstValue := csrfTokenPrefix +
		base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x11}, csrfTokenEntropyBytes))

	secondValue := csrfTokenPrefix +
		base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0x22}, csrfTokenEntropyBytes))

	firstToken, err := ParseCSRFToken(firstValue)
	if err != nil {
		t.Fatalf("parse first CSRF token: %v", err)
	}

	matchingToken, err := ParseCSRFToken(firstValue)
	if err != nil {
		t.Fatalf("parse matching CSRF token: %v", err)
	}

	differentToken, err := ParseCSRFToken(secondValue)
	if err != nil {
		t.Fatalf("parse different CSRF token: %v", err)
	}

	if !firstToken.Equal(matchingToken) {
		t.Fatal("matching CSRF tokens were not equal")
	}

	if firstToken.Equal(differentToken) {
		t.Fatal("different CSRF tokens were equal")
	}

	if firstToken.Equal(CSRFToken{}) {
		t.Fatal("valid CSRF token equaled a zero-value token")
	}
}

func TestCSRFTokenFormattingIsRedacted(t *testing.T) {
	t.Parallel()

	rawValue := csrfTokenPrefix +
		base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0xef}, csrfTokenEntropyBytes))

	token, err := ParseCSRFToken(rawValue)
	if err != nil {
		t.Fatalf("parse CSRF token: %v", err)
	}

	formattedValues := []string{
		fmt.Sprintf("%s", token),
		fmt.Sprintf("%v", token),
		fmt.Sprintf("%+v", token),
		fmt.Sprintf("%#v", token),
		fmt.Sprintf("%x", token),
	}

	for _, formattedValue := range formattedValues {
		if strings.Contains(formattedValue, rawValue) {
			t.Fatal("formatted CSRF token exposed its raw value")
		}

		if formattedValue != "[REDACTED]" {
			t.Fatalf("formatted value = %q, want [REDACTED]", formattedValue)
		}
	}
}

type csrfErrorReader struct {
	err error
}

func (reader csrfErrorReader) Read(_ []byte) (int, error) {
	return 0, reader.err
}
