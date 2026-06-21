package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/store"
)

type SessionSummary struct {
	ID        string
	UserAgent string
	CreatedAt time.Time
	ExpiresAt time.Time
	Current   bool
}

func (service *Service) ListSessions(
	ctx context.Context,
	principal Principal,
) ([]SessionSummary, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !service.managementAvailable() {
		return nil, ErrSessionUnavailable
	}

	if !validManagementPrincipal(principal) {
		return nil, ErrPrincipalInvalid
	}

	now := service.now().
		UTC().
		Truncate(time.Microsecond)

	storedSessions, err := service.sessions.ListActive(
		ctx,
		principal.UserID,
		now,
	)
	if err != nil {
		return nil, mapSessionManagementError(
			"list sessions",
			err,
		)
	}

	sessions := make(
		[]SessionSummary,
		0,
		len(storedSessions),
	)

	for _, storedSession := range storedSessions {
		if !validStoredSessionSummary(
			storedSession,
			now,
		) {
			return nil, fmt.Errorf(
				"list sessions: %w",
				ErrSessionUnavailable,
			)
		}

		sessions = append(
			sessions,
			SessionSummary{
				ID: storedSession.
					TokenFamilyID,
				UserAgent: storedSession.
					UserAgent,
				CreatedAt: storedSession.
					CreatedAt,
				ExpiresAt: storedSession.
					ExpiresAt,
				Current: storedSession.
					TokenFamilyID ==
					principal.SessionID,
			},
		)
	}

	return sessions, nil
}

func (service *Service) LogoutCurrent(
	ctx context.Context,
	principal Principal,
) error {
	err := service.RevokeSession(
		ctx,
		principal,
		principal.SessionID,
	)

	if errors.Is(err, ErrSessionNotFound) {
		return nil
	}

	return err
}

func (service *Service) RevokeSession(
	ctx context.Context,
	principal Principal,
	sessionID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !service.managementAvailable() {
		return ErrSessionUnavailable
	}

	if !validManagementPrincipal(principal) {
		return ErrPrincipalInvalid
	}

	if !validIdentifier(sessionID) {
		return ErrSessionNotFound
	}

	revokedAt := service.now().
		UTC().
		Truncate(time.Microsecond)

	err := service.sessions.RevokeOwnedFamily(
		ctx,
		principal.UserID,
		sessionID,
		revokedAt,
	)
	if err != nil {
		return mapSessionRevocationError(err)
	}

	return nil
}

func (service *Service) LogoutAll(
	ctx context.Context,
	principal Principal,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !service.managementAvailable() {
		return ErrSessionUnavailable
	}

	if !validManagementPrincipal(principal) {
		return ErrPrincipalInvalid
	}

	revokedAt := service.now().
		UTC().
		Truncate(time.Microsecond)

	err := service.sessions.RevokeAllForUser(
		ctx,
		principal.UserID,
		revokedAt,
	)
	if err != nil {
		return mapSessionManagementError(
			"revoke all sessions",
			err,
		)
	}

	return nil
}

func (service *Service) managementAvailable() bool {
	return service != nil &&
		service.sessions != nil &&
		service.now != nil
}

func validManagementPrincipal(
	principal Principal,
) bool {
	return validIdentifier(principal.UserID) &&
		validIdentifier(principal.SessionID)
}

func validStoredSessionSummary(
	summary store.SessionSummary,
	now time.Time,
) bool {
	return validIdentifier(
		summary.TokenFamilyID,
	) &&
		!summary.CreatedAt.IsZero() &&
		summary.ExpiresAt.After(now) &&
		summary.ExpiresAt.After(
			summary.CreatedAt,
		)
}

func mapSessionRevocationError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrSessionNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"revoke session: %w",
			ErrSessionUnavailable,
		)
	}
}

func mapSessionManagementError(
	operation string,
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"%s: %w",
			operation,
			ErrSessionUnavailable,
		)
	}
}
