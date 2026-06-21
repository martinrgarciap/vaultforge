package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

func (service *Service) AuthenticateAccessToken(
	ctx context.Context,
	tokenValue string,
) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}

	if !service.accessAuthenticationAvailable() {
		return Principal{}, ErrSessionUnavailable
	}

	principal, err := service.accessTokens.Verify(
		ctx,
		tokenValue,
	)
	if err != nil {
		return Principal{},
			mapAccessAuthenticationError(err)
	}

	if !validIdentifier(principal.UserID) ||
		!validIdentifier(principal.SessionID) {
		return Principal{}, ErrAccessTokenInvalid
	}

	now := service.now().
		UTC().
		Truncate(time.Microsecond)

	state, err := service.sessions.GetActiveState(
		ctx,
		principal.UserID,
		principal.SessionID,
		now,
	)
	if err != nil {
		return Principal{},
			mapActiveSessionAuthenticationError(err)
	}

	if !validAuthenticatedSessionState(
		state,
		principal,
		now,
	) {
		return Principal{},
			fmt.Errorf(
				"authenticate access token: %w",
				ErrSessionUnavailable,
			)
	}

	return principal, nil
}

func (service *Service) accessAuthenticationAvailable() bool {
	return service != nil &&
		service.sessions != nil &&
		service.accessTokens != nil &&
		service.now != nil
}

func validAuthenticatedSessionState(
	state store.SessionState,
	principal Principal,
	now time.Time,
) bool {
	return validIdentifier(state.UserID) &&
		validIdentifier(state.TokenFamilyID) &&
		state.UserID == principal.UserID &&
		state.TokenFamilyID == principal.SessionID &&
		!state.ExpiresAt.IsZero() &&
		state.ExpiresAt.After(now)
}

func mapAccessAuthenticationError(
	err error,
) error {
	switch {
	case errors.Is(err, ErrAccessTokenInvalid):
		return ErrAccessTokenInvalid

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"authenticate access token: %w",
			ErrSessionUnavailable,
		)
	}
}

func mapActiveSessionAuthenticationError(
	err error,
) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrAccessTokenInvalid

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"authenticate access token: %w",
			ErrSessionUnavailable,
		)
	}
}
