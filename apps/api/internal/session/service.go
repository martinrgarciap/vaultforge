package session

import (
	"context"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

type Authenticator interface {
	Login(
		ctx context.Context,
		input auth.LoginInput,
	) (auth.Account, error)
}

type SessionStore interface {
	Create(
		ctx context.Context,
		session *store.Session,
	) error

	GetActiveState(
		ctx context.Context,
		userID string,
		tokenFamilyID string,
		now time.Time,
	) (store.SessionState, error)

	RotateRefreshToken(
		ctx context.Context,
		currentRefreshTokenHash []byte,
		replacementRefreshTokenHash []byte,
		rotatedAt time.Time,
	) (store.SessionRotation, error)

	RevokeOwnedFamily(
		ctx context.Context,
		userID string,
		tokenFamilyID string,
		revokedAt time.Time,
	) error

	ListActive(
		ctx context.Context,
		userID string,
		now time.Time,
	) ([]store.SessionSummary, error)

	RevokeAllForUser(
		ctx context.Context,
		userID string,
		revokedAt time.Time,
	) error
}

type RefreshTokenProvider interface {
	Generate(
		ctx context.Context,
	) (RefreshToken, error)
}

type AccessTokenProvider interface {
	Issue(
		ctx context.Context,
		principal Principal,
	) (AccessToken, error)

	Verify(
		ctx context.Context,
		tokenValue string,
	) (Principal, error)
}

type Service struct {
	authenticator Authenticator
	sessions      SessionStore
	refreshTokens RefreshTokenProvider
	accessTokens  AccessTokenProvider
	lifetimes     TokenLifetimes
	now           func() time.Time
}

func NewService(
	authenticator Authenticator,
	sessions SessionStore,
	refreshTokens RefreshTokenProvider,
	accessTokens AccessTokenProvider,
	lifetimes TokenLifetimes,
) *Service {
	return &Service{
		authenticator: authenticator,
		sessions:      sessions,
		refreshTokens: refreshTokens,
		accessTokens:  accessTokens,
		lifetimes:     lifetimes,
		now:           time.Now,
	}
}

func (service *Service) IssueAccessToken(
	ctx context.Context,
	principal Principal,
) (AccessToken, error) {
	if err := ctx.Err(); err != nil {
		return AccessToken{}, err
	}

	if service == nil ||
		service.accessTokens == nil {
		return AccessToken{},
			ErrAccessTokenUnavailable
	}

	return service.accessTokens.Issue(
		ctx,
		principal,
	)
}

func (service *Service) VerifyAccessToken(
	ctx context.Context,
	tokenValue string,
) (Principal, error) {
	if err := ctx.Err(); err != nil {
		return Principal{}, err
	}

	if service == nil ||
		service.accessTokens == nil {
		return Principal{},
			ErrAccessTokenUnavailable
	}

	return service.accessTokens.Verify(
		ctx,
		tokenValue,
	)
}
