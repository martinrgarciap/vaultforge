package session

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTokenConfigCreatesAccessTokenManager(
	t *testing.T,
) {
	seed := bytes.Repeat(
		[]byte{0x71},
		ed25519.SeedSize,
	)

	lifetimes, err := NewTokenLifetimes(
		12*time.Minute,
		14*24*time.Hour,
		45*time.Second,
	)
	if err != nil {
		t.Fatalf(
			"create token lifetimes: %v",
			err,
		)
	}

	config, err := NewTokenConfig(
		"vaultforge-test-issuer",
		"vaultforge-test-audience",
		"test-ed25519-v1",
		seed,
		lifetimes,
	)
	if err != nil {
		t.Fatalf(
			"create token configuration: %v",
			err,
		)
	}

	if config.Issuer() !=
		"vaultforge-test-issuer" {
		t.Fatalf(
			"issuer = %q, want %q",
			config.Issuer(),
			"vaultforge-test-issuer",
		)
	}

	if config.Audience() !=
		"vaultforge-test-audience" {
		t.Fatalf(
			"audience = %q, want %q",
			config.Audience(),
			"vaultforge-test-audience",
		)
	}

	if config.KeyID() !=
		"test-ed25519-v1" {
		t.Fatalf(
			"key ID = %q, want %q",
			config.KeyID(),
			"test-ed25519-v1",
		)
	}

	if config.Lifetimes().AccessTokenTTL() !=
		12*time.Minute {
		t.Fatalf(
			"access-token TTL = %v, want %v",
			config.Lifetimes().
				AccessTokenTTL(),
			12*time.Minute,
		)
	}

	manager, err :=
		config.NewAccessTokenManager()
	if err != nil {
		t.Fatalf(
			"create access-token manager: %v",
			err,
		)
	}

	fixedTime := time.Date(
		2026,
		time.June,
		20,
		14,
		0,
		0,
		0,
		time.UTC,
	)

	manager.now = func() time.Time {
		return fixedTime
	}

	principal := Principal{
		UserID:    "user-123",
		SessionID: "session-family-456",
	}

	accessToken, err := manager.Issue(
		context.Background(),
		principal,
	)
	if err != nil {
		t.Fatalf(
			"issue access token: %v",
			err,
		)
	}

	verifiedPrincipal, err := manager.Verify(
		context.Background(),
		accessToken.Value(),
	)
	if err != nil {
		t.Fatalf(
			"verify access token: %v",
			err,
		)
	}

	if verifiedPrincipal != principal {
		t.Fatalf(
			"verified principal = %+v, want %+v",
			verifiedPrincipal,
			principal,
		)
	}
}

func TestNewTokenConfigRejectsInvalidValues(
	t *testing.T,
) {
	validSeed := bytes.Repeat(
		[]byte{0x72},
		ed25519.SeedSize,
	)

	validLifetimes := DefaultTokenLifetimes()

	testCases := []struct {
		name     string
		issuer   string
		audience string
		keyID    string
		seed     []byte
	}{
		{
			name:     "missing issuer",
			issuer:   "",
			audience: "vaultforge-api",
			keyID:    "local-ed25519-v1",
			seed:     validSeed,
		},
		{
			name:     "issuer contains whitespace",
			issuer:   "vaultforge api",
			audience: "vaultforge-api",
			keyID:    "local-ed25519-v1",
			seed:     validSeed,
		},
		{
			name:     "missing audience",
			issuer:   "vaultforge-api",
			audience: "",
			keyID:    "local-ed25519-v1",
			seed:     validSeed,
		},
		{
			name:     "audience contains whitespace",
			issuer:   "vaultforge-api",
			audience: "vaultforge api",
			keyID:    "local-ed25519-v1",
			seed:     validSeed,
		},
		{
			name:     "missing key ID",
			issuer:   "vaultforge-api",
			audience: "vaultforge-api",
			keyID:    "",
			seed:     validSeed,
		},
		{
			name:     "key ID contains whitespace",
			issuer:   "vaultforge-api",
			audience: "vaultforge-api",
			keyID:    "local key",
			seed:     validSeed,
		},
		{
			name:     "missing seed",
			issuer:   "vaultforge-api",
			audience: "vaultforge-api",
			keyID:    "local-ed25519-v1",
			seed:     nil,
		},
		{
			name:     "short seed",
			issuer:   "vaultforge-api",
			audience: "vaultforge-api",
			keyID:    "local-ed25519-v1",
			seed: []byte{
				0x01,
			},
		},
		{
			name:     "long seed",
			issuer:   "vaultforge-api",
			audience: "vaultforge-api",
			keyID:    "local-ed25519-v1",
			seed: bytes.Repeat(
				[]byte{0x73},
				ed25519.SeedSize+1,
			),
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				_, err := NewTokenConfig(
					testCase.issuer,
					testCase.audience,
					testCase.keyID,
					testCase.seed,
					validLifetimes,
				)

				if !errors.Is(
					err,
					ErrAccessTokenConfigurationInvalid,
				) {
					t.Fatalf(
						"expected ErrAccessTokenConfigurationInvalid, got %v",
						err,
					)
				}
			},
		)
	}
}

func TestNewTokenConfigRejectsInvalidLifetimes(
	t *testing.T,
) {
	_, err := NewTokenConfig(
		"vaultforge-api",
		"vaultforge-api",
		"local-ed25519-v1",
		bytes.Repeat(
			[]byte{0x74},
			ed25519.SeedSize,
		),
		TokenLifetimes{},
	)

	if !errors.Is(
		err,
		ErrAccessTokenTTLInvalid,
	) {
		t.Fatalf(
			"expected ErrAccessTokenTTLInvalid, got %v",
			err,
		)
	}
}

func TestZeroTokenConfigCannotCreateManager(
	t *testing.T,
) {
	var config TokenConfig

	_, err := config.NewAccessTokenManager()

	if !errors.Is(
		err,
		ErrAccessTokenConfigurationInvalid,
	) {
		t.Fatalf(
			"expected ErrAccessTokenConfigurationInvalid, got %v",
			err,
		)
	}
}

func TestTokenConfigFormattingIsRedacted(
	t *testing.T,
) {
	seed := bytes.Repeat(
		[]byte{0x75},
		ed25519.SeedSize,
	)

	config, err := NewTokenConfig(
		"vaultforge-api",
		"vaultforge-api",
		"local-ed25519-v1",
		seed,
		DefaultTokenLifetimes(),
	)
	if err != nil {
		t.Fatalf(
			"create token configuration: %v",
			err,
		)
	}

	privateKeyMarker := fmt.Sprintf(
		"%x",
		ed25519.NewKeyFromSeed(seed),
	)

	formattedValues := []string{
		fmt.Sprintf("%s", config),
		fmt.Sprintf("%v", config),
		fmt.Sprintf("%+v", config),
		fmt.Sprintf("%#v", config),
		fmt.Sprintf("%x", config),
	}

	for _, formattedValue := range formattedValues {
		if strings.Contains(
			formattedValue,
			privateKeyMarker,
		) {
			t.Fatal(
				"formatted token configuration exposed the private key",
			)
		}

		if formattedValue != "[REDACTED]" {
			t.Fatalf(
				"formatted value = %q, want [REDACTED]",
				formattedValue,
			)
		}
	}
}
