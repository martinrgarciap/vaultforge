package session

import "time"

const (
	DefaultAccessTokenTTL  = 10 * time.Minute
	DefaultRefreshTokenTTL = 30 * 24 * time.Hour
	DefaultClockLeeway     = 30 * time.Second
)

type TokenLifetimes struct {
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
	clockLeeway     time.Duration
}

func NewTokenLifetimes(
	accessTokenTTL time.Duration,
	refreshTokenTTL time.Duration,
	clockLeeway time.Duration,
) (TokenLifetimes, error) {
	switch {
	case accessTokenTTL <= 0:
		return TokenLifetimes{},
			ErrAccessTokenTTLInvalid

	case refreshTokenTTL <= accessTokenTTL:
		return TokenLifetimes{},
			ErrRefreshTokenTTLInvalid

	case clockLeeway < 0 ||
		clockLeeway >= accessTokenTTL:
		return TokenLifetimes{},
			ErrClockLeewayInvalid
	}

	return TokenLifetimes{
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
		clockLeeway:     clockLeeway,
	}, nil
}

func DefaultTokenLifetimes() TokenLifetimes {
	return TokenLifetimes{
		accessTokenTTL:  DefaultAccessTokenTTL,
		refreshTokenTTL: DefaultRefreshTokenTTL,
		clockLeeway:     DefaultClockLeeway,
	}
}

func (lifetimes TokenLifetimes) Validate() error {
	_, err := NewTokenLifetimes(
		lifetimes.accessTokenTTL,
		lifetimes.refreshTokenTTL,
		lifetimes.clockLeeway,
	)

	return err
}

func (lifetimes TokenLifetimes) AccessTokenTTL() time.Duration {
	return lifetimes.accessTokenTTL
}

func (lifetimes TokenLifetimes) RefreshTokenTTL() time.Duration {
	return lifetimes.refreshTokenTTL
}

func (lifetimes TokenLifetimes) ClockLeeway() time.Duration {
	return lifetimes.clockLeeway
}
