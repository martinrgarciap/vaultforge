package session

import (
	"errors"
	"testing"
	"time"
)

func TestDefaultTokenLifetimes(t *testing.T) {
	lifetimes := DefaultTokenLifetimes()

	if err := lifetimes.Validate(); err != nil {
		t.Fatalf(
			"default token lifetimes should be valid: %v",
			err,
		)
	}

	if lifetimes.AccessTokenTTL() !=
		DefaultAccessTokenTTL {
		t.Errorf(
			"access token TTL = %v, want %v",
			lifetimes.AccessTokenTTL(),
			DefaultAccessTokenTTL,
		)
	}

	if lifetimes.RefreshTokenTTL() !=
		DefaultRefreshTokenTTL {
		t.Errorf(
			"refresh token TTL = %v, want %v",
			lifetimes.RefreshTokenTTL(),
			DefaultRefreshTokenTTL,
		)
	}

	if lifetimes.ClockLeeway() !=
		DefaultClockLeeway {
		t.Errorf(
			"clock leeway = %v, want %v",
			lifetimes.ClockLeeway(),
			DefaultClockLeeway,
		)
	}
}

func TestNewTokenLifetimes(t *testing.T) {
	lifetimes, err := NewTokenLifetimes(
		15*time.Minute,
		14*24*time.Hour,
		45*time.Second,
	)
	if err != nil {
		t.Fatalf(
			"create token lifetimes: %v",
			err,
		)
	}

	if lifetimes.AccessTokenTTL() !=
		15*time.Minute {
		t.Errorf(
			"access token TTL = %v, want %v",
			lifetimes.AccessTokenTTL(),
			15*time.Minute,
		)
	}

	if lifetimes.RefreshTokenTTL() !=
		14*24*time.Hour {
		t.Errorf(
			"refresh token TTL = %v, want %v",
			lifetimes.RefreshTokenTTL(),
			14*24*time.Hour,
		)
	}

	if lifetimes.ClockLeeway() !=
		45*time.Second {
		t.Errorf(
			"clock leeway = %v, want %v",
			lifetimes.ClockLeeway(),
			45*time.Second,
		)
	}
}

func TestNewTokenLifetimesRejectsInvalidValues(
	t *testing.T,
) {
	testCases := []struct {
		name            string
		accessTokenTTL  time.Duration
		refreshTokenTTL time.Duration
		clockLeeway     time.Duration
		expectedError   error
	}{
		{
			name:            "zero access token lifetime",
			accessTokenTTL:  0,
			refreshTokenTTL: 24 * time.Hour,
			clockLeeway:     30 * time.Second,
			expectedError:   ErrAccessTokenTTLInvalid,
		},
		{
			name:            "negative access token lifetime",
			accessTokenTTL:  -time.Minute,
			refreshTokenTTL: 24 * time.Hour,
			clockLeeway:     30 * time.Second,
			expectedError:   ErrAccessTokenTTLInvalid,
		},
		{
			name:            "equal refresh and access lifetimes",
			accessTokenTTL:  10 * time.Minute,
			refreshTokenTTL: 10 * time.Minute,
			clockLeeway:     30 * time.Second,
			expectedError:   ErrRefreshTokenTTLInvalid,
		},
		{
			name:            "refresh lifetime shorter than access lifetime",
			accessTokenTTL:  10 * time.Minute,
			refreshTokenTTL: 5 * time.Minute,
			clockLeeway:     30 * time.Second,
			expectedError:   ErrRefreshTokenTTLInvalid,
		},
		{
			name:            "negative clock leeway",
			accessTokenTTL:  10 * time.Minute,
			refreshTokenTTL: 24 * time.Hour,
			clockLeeway:     -time.Second,
			expectedError:   ErrClockLeewayInvalid,
		},
		{
			name:            "clock leeway equals access lifetime",
			accessTokenTTL:  10 * time.Minute,
			refreshTokenTTL: 24 * time.Hour,
			clockLeeway:     10 * time.Minute,
			expectedError:   ErrClockLeewayInvalid,
		},
		{
			name:            "clock leeway exceeds access lifetime",
			accessTokenTTL:  10 * time.Minute,
			refreshTokenTTL: 24 * time.Hour,
			clockLeeway:     11 * time.Minute,
			expectedError:   ErrClockLeewayInvalid,
		},
	}

	for _, testCase := range testCases {
		t.Run(
			testCase.name,
			func(t *testing.T) {
				_, err := NewTokenLifetimes(
					testCase.accessTokenTTL,
					testCase.refreshTokenTTL,
					testCase.clockLeeway,
				)

				if !errors.Is(
					err,
					testCase.expectedError,
				) {
					t.Fatalf(
						"expected %v, got %v",
						testCase.expectedError,
						err,
					)
				}
			},
		)
	}
}

func TestZeroTokenLifetimesAreInvalid(
	t *testing.T,
) {
	var lifetimes TokenLifetimes

	err := lifetimes.Validate()

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
