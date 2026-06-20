package session

import (
	"crypto/ed25519"
	"fmt"
	"io"
)

type TokenConfig struct {
	issuer     string
	audience   string
	keyID      string
	privateKey ed25519.PrivateKey
	lifetimes  TokenLifetimes
}

func NewTokenConfig(
	issuer string,
	audience string,
	keyID string,
	seed []byte,
	lifetimes TokenLifetimes,
) (TokenConfig, error) {
	if !validConfigurationValue(issuer) ||
		!validConfigurationValue(audience) ||
		!validConfigurationValue(keyID) ||
		len(seed) != ed25519.SeedSize {
		return TokenConfig{},
			ErrAccessTokenConfigurationInvalid
	}

	if err := lifetimes.Validate(); err != nil {
		return TokenConfig{},
			fmt.Errorf(
				"create token configuration: %w",
				err,
			)
	}

	privateKey := ed25519.NewKeyFromSeed(seed)

	privateKeyCopy := append(
		ed25519.PrivateKey(nil),
		privateKey...,
	)

	return TokenConfig{
		issuer:     issuer,
		audience:   audience,
		keyID:      keyID,
		privateKey: privateKeyCopy,
		lifetimes:  lifetimes,
	}, nil
}

func (config TokenConfig) NewAccessTokenManager() (
	*AccessTokenManager,
	error,
) {
	return NewAccessTokenManager(
		config.issuer,
		config.audience,
		config.keyID,
		config.privateKey,
		config.lifetimes,
	)
}

func (config TokenConfig) Issuer() string {
	return config.issuer
}

func (config TokenConfig) Audience() string {
	return config.audience
}

func (config TokenConfig) KeyID() string {
	return config.keyID
}

func (config TokenConfig) Lifetimes() TokenLifetimes {
	return config.lifetimes
}

func (config TokenConfig) Format(
	state fmt.State,
	_ rune,
) {
	_, _ = io.WriteString(
		state,
		"[REDACTED]",
	)
}
