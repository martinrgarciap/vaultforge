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

	"github.com/golang-jwt/jwt/v5"
)

const (
	testAccessTokenIssuer   = "vaultforge-api"
	testAccessTokenAudience = "vaultforge-api"
	testAccessTokenKeyID    = "test-ed25519-v1"
	testAccessTokenUserID   = "user-123"
	testAccessTokenSession  = "session-family-456"
)

var testAccessTokenTime = time.Date(
	2026,
	time.June,
	20,
	12,
	0,
	0,
	0,
	time.UTC,
)

func TestAccessTokenManagerIssuesAndVerifiesToken(
	t *testing.T,
) {
	manager, _ := newTestAccessTokenManager(t)

	principal := Principal{
		UserID:    testAccessTokenUserID,
		SessionID: testAccessTokenSession,
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

	if accessToken.Value() == "" {
		t.Fatal("expected a nonempty access token")
	}

	expectedExpiry := testAccessTokenTime.Add(
		DefaultAccessTokenTTL,
	)

	if !accessToken.ExpiresAt().Equal(
		expectedExpiry,
	) {
		t.Fatalf(
			"access-token expiry = %v, want %v",
			accessToken.ExpiresAt(),
			expectedExpiry,
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

func TestAccessTokenManagerRejectsInvalidTokens(
	t *testing.T,
) {
	manager, privateKey :=
		newTestAccessTokenManager(t)

	baseClaims := validTestAccessTokenClaims()

	expiredClaims := baseClaims
	expiredClaims.ExpiresAt = jwt.NewNumericDate(
		testAccessTokenTime.Add(-2 * time.Minute),
	)

	notYetValidClaims := baseClaims
	notYetValidClaims.NotBefore = jwt.NewNumericDate(
		testAccessTokenTime.Add(2 * time.Minute),
	)

	wrongIssuerClaims := baseClaims
	wrongIssuerClaims.Issuer = "other-issuer"

	wrongAudienceClaims := baseClaims
	wrongAudienceClaims.Audience = jwt.ClaimStrings{
		"other-audience",
	}

	missingExpirationClaims := baseClaims
	missingExpirationClaims.ExpiresAt = nil

	missingNotBeforeClaims := baseClaims
	missingNotBeforeClaims.NotBefore = nil

	missingIssuedAtClaims := baseClaims
	missingIssuedAtClaims.IssuedAt = nil

	missingSubjectClaims := baseClaims
	missingSubjectClaims.Subject = ""

	missingSessionClaims := baseClaims
	missingSessionClaims.SessionID = ""

	alternatePrivateKey := ed25519.NewKeyFromSeed(
		bytes.Repeat(
			[]byte{0x99},
			ed25519.SeedSize,
		),
	)

	testCases := map[string]string{
		"empty":     "",
		"malformed": "not-a-jwt",
		"expired": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			expiredClaims,
		),
		"not yet valid": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			notYetValidClaims,
		),
		"wrong issuer": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			wrongIssuerClaims,
		),
		"wrong audience": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			wrongAudienceClaims,
		),
		"missing expiration": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			missingExpirationClaims,
		),
		"missing not before": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			missingNotBeforeClaims,
		),
		"missing issued at": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			missingIssuedAtClaims,
		),
		"missing subject": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			missingSubjectClaims,
		),
		"missing session ID": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			testAccessTokenKeyID,
			missingSessionClaims,
		),
		"wrong key ID": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			"other-key",
			baseClaims,
		),
		"missing key ID": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			privateKey,
			"",
			baseClaims,
		),
		"invalid signature": signTestAccessToken(
			t,
			jwt.SigningMethodEdDSA,
			alternatePrivateKey,
			testAccessTokenKeyID,
			baseClaims,
		),
		"unsupported algorithm": signTestAccessToken(
			t,
			jwt.SigningMethodHS256,
			[]byte("synthetic-test-signing-key"),
			testAccessTokenKeyID,
			baseClaims,
		),
	}

	for name, tokenValue := range testCases {
		t.Run(
			name,
			func(t *testing.T) {
				_, err := manager.Verify(
					context.Background(),
					tokenValue,
				)

				if !errors.Is(
					err,
					ErrAccessTokenInvalid,
				) {
					t.Fatalf(
						"expected ErrAccessTokenInvalid, got %v",
						err,
					)
				}
			},
		)
	}
}

func TestAccessTokenManagerRejectsInvalidPrincipal(
	t *testing.T,
) {
	manager, _ := newTestAccessTokenManager(t)

	testCases := []Principal{
		{},
		{
			UserID:    testAccessTokenUserID,
			SessionID: "",
		},
		{
			UserID:    "",
			SessionID: testAccessTokenSession,
		},
		{
			UserID:    "user with spaces",
			SessionID: testAccessTokenSession,
		},
	}

	for _, principal := range testCases {
		_, err := manager.Issue(
			context.Background(),
			principal,
		)

		if !errors.Is(
			err,
			ErrPrincipalInvalid,
		) {
			t.Fatalf(
				"expected ErrPrincipalInvalid, got %v",
				err,
			)
		}
	}
}

func TestAccessTokenManagerHonorsContextCancellation(
	t *testing.T,
) {
	manager, _ := newTestAccessTokenManager(t)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := manager.Issue(
		ctx,
		Principal{
			UserID:    testAccessTokenUserID,
			SessionID: testAccessTokenSession,
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}

	_, err = manager.Verify(
		ctx,
		"synthetic-token",
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"expected context cancellation, got %v",
			err,
		)
	}
}

func TestNewAccessTokenManagerRejectsInvalidConfiguration(
	t *testing.T,
) {
	privateKey := testEd25519PrivateKey()
	lifetimes := DefaultTokenLifetimes()

	testCases := []struct {
		name       string
		issuer     string
		audience   string
		keyID      string
		privateKey ed25519.PrivateKey
	}{
		{
			name:       "missing issuer",
			issuer:     "",
			audience:   testAccessTokenAudience,
			keyID:      testAccessTokenKeyID,
			privateKey: privateKey,
		},
		{
			name:       "missing audience",
			issuer:     testAccessTokenIssuer,
			audience:   "",
			keyID:      testAccessTokenKeyID,
			privateKey: privateKey,
		},
		{
			name:       "missing key ID",
			issuer:     testAccessTokenIssuer,
			audience:   testAccessTokenAudience,
			keyID:      "",
			privateKey: privateKey,
		},
		{
			name:       "key ID contains whitespace",
			issuer:     testAccessTokenIssuer,
			audience:   testAccessTokenAudience,
			keyID:      "invalid key",
			privateKey: privateKey,
		},
		{
			name:     "invalid private key",
			issuer:   testAccessTokenIssuer,
			audience: testAccessTokenAudience,
			keyID:    testAccessTokenKeyID,
			privateKey: ed25519.PrivateKey{
				0x01,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				_, err := NewAccessTokenManager(
					testCase.issuer,
					testCase.audience,
					testCase.keyID,
					testCase.privateKey,
					lifetimes,
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

func TestNewAccessTokenManagerRejectsInvalidLifetimes(
	t *testing.T,
) {
	_, err := NewAccessTokenManager(
		testAccessTokenIssuer,
		testAccessTokenAudience,
		testAccessTokenKeyID,
		testEd25519PrivateKey(),
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

func TestAccessTokenFormattingIsRedacted(
	t *testing.T,
) {
	manager, _ := newTestAccessTokenManager(t)

	accessToken, err := manager.Issue(
		context.Background(),
		Principal{
			UserID:    testAccessTokenUserID,
			SessionID: testAccessTokenSession,
		},
	)
	if err != nil {
		t.Fatalf(
			"issue access token: %v",
			err,
		)
	}

	formattedValues := []string{
		fmt.Sprintf("%s", accessToken),
		fmt.Sprintf("%v", accessToken),
		fmt.Sprintf("%+v", accessToken),
		fmt.Sprintf("%#v", accessToken),
		fmt.Sprintf("%v", manager),
		fmt.Sprintf("%+v", manager),
	}

	for _, formattedValue := range formattedValues {
		if strings.Contains(
			formattedValue,
			accessToken.Value(),
		) {
			t.Fatal(
				"formatted value exposed an access token",
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

func newTestAccessTokenManager(
	t *testing.T,
) (*AccessTokenManager, ed25519.PrivateKey) {
	t.Helper()

	privateKey := testEd25519PrivateKey()

	manager, err := NewAccessTokenManager(
		testAccessTokenIssuer,
		testAccessTokenAudience,
		testAccessTokenKeyID,
		privateKey,
		DefaultTokenLifetimes(),
	)
	if err != nil {
		t.Fatalf(
			"create access token manager: %v",
			err,
		)
	}

	manager.now = func() time.Time {
		return testAccessTokenTime
	}

	return manager, privateKey
}

func testEd25519PrivateKey() ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(
		bytes.Repeat(
			[]byte{0x42},
			ed25519.SeedSize,
		),
	)
}

func validTestAccessTokenClaims() accessTokenClaims {
	return accessTokenClaims{
		SessionID: testAccessTokenSession,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  testAccessTokenIssuer,
			Subject: testAccessTokenUserID,
			Audience: jwt.ClaimStrings{
				testAccessTokenAudience,
			},
			ExpiresAt: jwt.NewNumericDate(
				testAccessTokenTime.Add(
					DefaultAccessTokenTTL,
				),
			),
			NotBefore: jwt.NewNumericDate(
				testAccessTokenTime,
			),
			IssuedAt: jwt.NewNumericDate(
				testAccessTokenTime,
			),
		},
	}
}

func signTestAccessToken(
	t *testing.T,
	method jwt.SigningMethod,
	signingKey any,
	keyID string,
	claims accessTokenClaims,
) string {
	t.Helper()

	token := jwt.NewWithClaims(
		method,
		claims,
	)

	if keyID != "" {
		token.Header["kid"] = keyID
	}

	tokenValue, err := token.SignedString(
		signingKey,
	)
	if err != nil {
		t.Fatalf(
			"sign synthetic access token: %v",
			err,
		)
	}

	return tokenValue
}
