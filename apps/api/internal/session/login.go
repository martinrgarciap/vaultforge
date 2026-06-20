package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

const maxStoredUserAgentRunes = 512

type LoginInput struct {
	Email     string
	Password  string
	UserAgent string
}

type LoginResult struct {
	Account               auth.Account
	AccessToken           AccessToken
	RefreshToken          RefreshToken
	RefreshTokenExpiresAt time.Time
}

func (service *Service) Login(
	ctx context.Context,
	input LoginInput,
) (LoginResult, error) {
	if err := ctx.Err(); err != nil {
		return LoginResult{}, err
	}

	if !service.loginAvailable() {
		return LoginResult{},
			ErrSessionUnavailable
	}

	account, err := service.authenticator.Login(
		ctx,
		auth.LoginInput{
			Email:    input.Email,
			Password: input.Password,
		},
	)
	if err != nil {
		return LoginResult{},
			mapLoginAuthenticationError(err)
	}

	if !validIdentifier(account.ID) {
		return LoginResult{},
			ErrSessionUnavailable
	}

	refreshToken, err :=
		service.refreshTokens.Generate(ctx)
	if err != nil {
		return LoginResult{},
			mapLoginOperationError(err)
	}

	refreshTokenDigest, err :=
		refreshToken.Digest()
	if err != nil {
		return LoginResult{},
			ErrSessionUnavailable
	}

	createdAt := service.now().
		UTC().
		Truncate(time.Microsecond)

	refreshTokenExpiresAt := createdAt.Add(
		service.lifetimes.RefreshTokenTTL(),
	)

	storedSession := &store.Session{
		UserID: account.ID,
		RefreshTokenHash: refreshTokenDigest.
			Bytes(),
		ExpiresAt: refreshTokenExpiresAt,
		UserAgent: normalizeUserAgent(
			input.UserAgent,
		),
	}

	if err := service.sessions.Create(
		ctx,
		storedSession,
	); err != nil {
		return LoginResult{},
			mapLoginOperationError(err)
	}

	revokedAt := storedSession.CreatedAt

	if revokedAt.IsZero() {
		revokedAt = service.now().
			UTC().
			Truncate(time.Microsecond)
	}

	if !validIdentifier(
		storedSession.TokenFamilyID,
	) {
		service.revokeCreatedFamily(
			ctx,
			account.ID,
			storedSession.TokenFamilyID,
			revokedAt,
		)

		return LoginResult{},
			ErrSessionUnavailable
	}

	accessToken, err := service.accessTokens.Issue(
		ctx,
		Principal{
			UserID:    account.ID,
			SessionID: storedSession.TokenFamilyID,
		},
	)
	if err != nil {
		service.revokeCreatedFamily(
			ctx,
			account.ID,
			storedSession.TokenFamilyID,
			revokedAt,
		)

		return LoginResult{},
			mapLoginOperationError(err)
	}

	if err := ctx.Err(); err != nil {
		service.revokeCreatedFamily(
			ctx,
			account.ID,
			storedSession.TokenFamilyID,
			revokedAt,
		)

		return LoginResult{}, err
	}

	return LoginResult{
		Account:               account,
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: refreshTokenExpiresAt,
	}, nil
}

func (service *Service) loginAvailable() bool {
	return service != nil &&
		service.authenticator != nil &&
		service.sessions != nil &&
		service.refreshTokens != nil &&
		service.accessTokens != nil &&
		service.now != nil &&
		service.lifetimes.Validate() == nil
}

func (service *Service) revokeCreatedFamily(
	ctx context.Context,
	userID string,
	tokenFamilyID string,
	revokedAt time.Time,
) {
	if service == nil ||
		service.sessions == nil ||
		!validIdentifier(userID) ||
		!validIdentifier(tokenFamilyID) {
		return
	}

	cleanupContext := context.WithoutCancel(
		ctx,
	)

	_ = service.sessions.RevokeOwnedFamily(
		cleanupContext,
		userID,
		tokenFamilyID,
		revokedAt,
	)
}

func normalizeUserAgent(
	value string,
) *string {
	normalized := strings.TrimSpace(
		strings.ToValidUTF8(value, ""),
	)

	if normalized == "" {
		return nil
	}

	runes := []rune(normalized)

	if len(runes) >
		maxStoredUserAgentRunes {
		normalized = string(
			runes[:maxStoredUserAgentRunes],
		)
	}

	return &normalized
}

func mapLoginAuthenticationError(
	err error,
) error {
	switch {
	case errors.Is(
		err,
		auth.ErrInvalidCredentials,
	):
		return auth.ErrInvalidCredentials

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"create login session: %w",
			ErrSessionUnavailable,
		)
	}
}

func mapLoginOperationError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"create login session: %w",
			ErrSessionUnavailable,
		)
	}
}
