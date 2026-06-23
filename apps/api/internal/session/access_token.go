package session

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	maxAccessTokenLength = 4096
	maxIdentifierBytes   = 256
)

type Principal struct {
	UserID    string
	SessionID string
}

type AccessToken struct {
	value     string
	expiresAt time.Time
}

type accessTokenClaims struct {
	SessionID string `json:"sid"`
	jwt.RegisteredClaims
}

type AccessTokenManager struct {
	issuer     string
	audience   string
	keyID      string
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	lifetimes  TokenLifetimes
	now        func() time.Time
}

func NewAccessTokenManager(
	issuer string,
	audience string,
	keyID string,
	privateKey ed25519.PrivateKey,
	lifetimes TokenLifetimes,
) (*AccessTokenManager, error) {
	if !validConfigurationValue(issuer) ||
		!validConfigurationValue(audience) ||
		!validConfigurationValue(keyID) ||
		len(privateKey) != ed25519.PrivateKeySize {
		return nil, ErrAccessTokenConfigurationInvalid
	}

	if err := lifetimes.Validate(); err != nil {
		return nil, fmt.Errorf(
			"create access token manager: %w",
			err,
		)
	}

	privateKeyCopy := append(
		ed25519.PrivateKey(nil),
		privateKey...,
	)

	publicKey, ok := privateKeyCopy.Public().(ed25519.PublicKey)
	if !ok ||
		len(publicKey) != ed25519.PublicKeySize {
		return nil, ErrAccessTokenConfigurationInvalid
	}

	publicKeyCopy := append(
		ed25519.PublicKey(nil),
		publicKey...,
	)

	return &AccessTokenManager{
		issuer:     issuer,
		audience:   audience,
		keyID:      keyID,
		privateKey: privateKeyCopy,
		publicKey:  publicKeyCopy,
		lifetimes:  lifetimes,
		now:        time.Now,
	}, nil
}

func (manager *AccessTokenManager) Issue(
	ctx context.Context,
	principal Principal,
) (AccessToken, error) {
	if err := ctx.Err(); err != nil {
		return AccessToken{}, err
	}

	if !manager.valid() {
		return AccessToken{},
			ErrAccessTokenUnavailable
	}

	if !validIdentifier(principal.UserID) ||
		!validIdentifier(principal.SessionID) {
		return AccessToken{}, ErrPrincipalInvalid
	}

	issuedAt := manager.now().
		UTC().
		Truncate(time.Second)

	expiresAt := issuedAt.Add(
		manager.lifetimes.AccessTokenTTL(),
	)

	claims := accessTokenClaims{
		SessionID: principal.SessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:  manager.issuer,
			Subject: principal.UserID,
			Audience: jwt.ClaimStrings{
				manager.audience,
			},
			ExpiresAt: jwt.NewNumericDate(
				expiresAt,
			),
			NotBefore: jwt.NewNumericDate(
				issuedAt,
			),
			IssuedAt: jwt.NewNumericDate(
				issuedAt,
			),
		},
	}

	token := jwt.NewWithClaims(
		jwt.SigningMethodEdDSA,
		claims,
	)

	token.Header["kid"] = manager.keyID

	tokenValue, err := token.SignedString(
		manager.privateKey,
	)
	if err != nil {
		return AccessToken{},
			ErrAccessTokenUnavailable
	}

	if err := ctx.Err(); err != nil {
		return AccessToken{}, err
	}

	return AccessToken{
		value:     tokenValue,
		expiresAt: expiresAt,
	}, nil
}

func (manager *AccessTokenManager) Verify(
	ctx context.Context,
	tokenValue string,
) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}

	if !manager.valid() {
		return Principal{},
			ErrAccessTokenUnavailable
	}

	if tokenValue == "" ||
		len(tokenValue) > maxAccessTokenLength {
		return Principal{}, ErrAccessTokenInvalid
	}

	claims := &accessTokenClaims{}

	token, err := jwt.ParseWithClaims(
		tokenValue,
		claims,
		func(token *jwt.Token) (any, error) {
			if token == nil ||
				token.Method == nil ||
				token.Method.Alg() !=
					jwt.SigningMethodEdDSA.Alg() {
				return nil, ErrAccessTokenInvalid
			}

			keyID, ok := token.Header["kid"].(string)
			if !ok || keyID != manager.keyID {
				return nil, ErrAccessTokenInvalid
			}

			return manager.publicKey, nil
		},
		jwt.WithValidMethods(
			[]string{
				jwt.SigningMethodEdDSA.Alg(),
			},
		),
		jwt.WithIssuer(manager.issuer),
		jwt.WithAudience(manager.audience),
		jwt.WithExpirationRequired(),
		jwt.WithNotBeforeRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(
			manager.lifetimes.ClockLeeway(),
		),
		jwt.WithTimeFunc(manager.now),
		jwt.WithStrictDecoding(),
	)
	if err != nil ||
		token == nil ||
		!token.Valid {
		return Principal{}, ErrAccessTokenInvalid
	}

	if claims.IssuedAt == nil ||
		!validIdentifier(claims.Subject) ||
		!validIdentifier(claims.SessionID) {
		return Principal{}, ErrAccessTokenInvalid
	}

	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}

	return Principal{
		UserID:    claims.Subject,
		SessionID: claims.SessionID,
	}, nil
}

func (token AccessToken) Value() string {
	return token.value
}

func (token AccessToken) ExpiresAt() time.Time {
	return token.expiresAt
}

func (token AccessToken) Format(
	state fmt.State,
	_ rune,
) {
	_, _ = io.WriteString(
		state,
		"[REDACTED]",
	)
}

func (manager *AccessTokenManager) Format(
	state fmt.State,
	_ rune,
) {
	_, _ = io.WriteString(
		state,
		"[REDACTED]",
	)
}

func (manager *AccessTokenManager) valid() bool {
	return manager != nil &&
		validConfigurationValue(manager.issuer) &&
		validConfigurationValue(manager.audience) &&
		validConfigurationValue(manager.keyID) &&
		len(manager.privateKey) ==
			ed25519.PrivateKeySize &&
		len(manager.publicKey) ==
			ed25519.PublicKeySize &&
		manager.now != nil &&
		manager.lifetimes.Validate() == nil
}

func validConfigurationValue(
	value string,
) bool {
	return value != "" &&
		!strings.ContainsAny(
			value,
			" \t\r\n",
		)
}

func validIdentifier(value string) bool {
	return value != "" &&
		len(value) <= maxIdentifierBytes &&
		!strings.ContainsAny(
			value,
			" \t\r\n",
		)
}
