package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

type RefreshInput struct {
	RefreshToken string
}

type RefreshResult struct {
	AccessToken           AccessToken
	RefreshToken          RefreshToken
	RefreshTokenExpiresAt time.Time
}

func (service *Service) Refresh(
	ctx context.Context,
	input RefreshInput,
) (RefreshResult, error) {
	if err := ctx.Err(); err != nil {
		return RefreshResult{}, err
	}

	if !service.refreshAvailable() {
		return RefreshResult{},
			ErrSessionUnavailable
	}

	currentRefreshToken, err :=
		ParseRefreshToken(input.RefreshToken)
	if err != nil {
		return RefreshResult{},
			ErrRefreshTokenInvalid
	}

	currentRefreshTokenDigest, err :=
		currentRefreshToken.Digest()
	if err != nil {
		return RefreshResult{},
			ErrRefreshTokenInvalid
	}

	replacementRefreshToken, err :=
		service.refreshTokens.Generate(ctx)
	if err != nil {
		return RefreshResult{},
			mapRefreshOperationError(err)
	}

	replacementRefreshTokenDigest, err :=
		replacementRefreshToken.Digest()
	if err != nil {
		return RefreshResult{},
			fmt.Errorf(
				"refresh session: %w",
				ErrSessionUnavailable,
			)
	}

	rotatedAt := service.now().
		UTC().
		Truncate(time.Microsecond)

	rotation, err :=
		service.sessions.RotateRefreshToken(
			ctx,
			currentRefreshTokenDigest.Bytes(),
			replacementRefreshTokenDigest.Bytes(),
			rotatedAt,
		)
	if err != nil {
		return RefreshResult{},
			mapRefreshRotationError(err)
	}

	if !validRefreshRotation(
		rotation,
		rotatedAt,
	) {
		service.revokeRotatedFamily(
			ctx,
			rotation,
		)

		return RefreshResult{},
			fmt.Errorf(
				"refresh session: %w",
				ErrSessionUnavailable,
			)
	}

	accessToken, err :=
		service.accessTokens.Issue(
			ctx,
			Principal{
				UserID: rotation.UserID,
				SessionID: rotation.
					TokenFamilyID,
			},
		)
	if err != nil {
		service.revokeRotatedFamily(
			ctx,
			rotation,
		)

		return RefreshResult{},
			mapRefreshOperationError(err)
	}

	if err := ctx.Err(); err != nil {
		service.revokeRotatedFamily(
			ctx,
			rotation,
		)

		return RefreshResult{}, err
	}

	return RefreshResult{
		AccessToken:           accessToken,
		RefreshToken:          replacementRefreshToken,
		RefreshTokenExpiresAt: rotation.ExpiresAt,
	}, nil
}

func (service *Service) refreshAvailable() bool {
	return service != nil &&
		service.sessions != nil &&
		service.refreshTokens != nil &&
		service.accessTokens != nil &&
		service.now != nil
}

func validRefreshRotation(
	rotation store.SessionRotation,
	rotatedAt time.Time,
) bool {
	return validIdentifier(rotation.UserID) &&
		validIdentifier(rotation.TokenFamilyID) &&
		!rotation.CreatedAt.IsZero() &&
		rotation.ExpiresAt.After(rotatedAt)
}

func (service *Service) revokeRotatedFamily(
	ctx context.Context,
	rotation store.SessionRotation,
) {
	revokedAt := rotation.CreatedAt

	if revokedAt.IsZero() &&
		service != nil &&
		service.now != nil {
		revokedAt = service.now().
			UTC().
			Truncate(time.Microsecond)
	}

	service.revokeCreatedFamily(
		ctx,
		rotation.UserID,
		rotation.TokenFamilyID,
		revokedAt,
	)
}

func mapRefreshRotationError(
	err error,
) error {
	switch {
	case errors.Is(err, store.ErrNotFound),
		errors.Is(
			err,
			store.ErrSessionReplayDetected,
		):
		return ErrRefreshTokenInvalid

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"refresh session: %w",
			ErrSessionUnavailable,
		)
	}
}

func mapRefreshOperationError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"refresh session: %w",
			ErrSessionUnavailable,
		)
	}
}
