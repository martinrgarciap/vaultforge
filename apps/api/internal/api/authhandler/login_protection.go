package authhandler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
)

const invalidLoginEmailIdentity = "invalid-email"

type LoginProtector interface {
	Check(
		ctx context.Context,
		identityParts ...string,
	) (ratelimit.LockoutDecision, error)

	RecordFailure(
		ctx context.Context,
		identityParts ...string,
	) (ratelimit.LockoutDecision, error)

	Clear(
		ctx context.Context,
		identityParts ...string,
	) error
}

func loginProtectionIdentity(
	r *http.Request,
	email string,
) ([]string, bool) {
	peerAddress, ok := appmiddleware.PeerIP(r)
	if !ok {
		return nil, false
	}

	normalizedEmail, err := auth.NormalizeEmail(email)
	if err != nil {
		normalizedEmail = invalidLoginEmailIdentity
	}

	return []string{
		normalizedEmail,
		peerAddress,
	}, true
}

func (handler *Handler) writeLoginLockout(
	w http.ResponseWriter,
	r *http.Request,
	retryAfter time.Duration,
) {
	w.Header().Set(
		"Retry-After",
		strconv.FormatInt(
			loginRetryAfterSeconds(retryAfter),
			10,
		),
	)

	handler.writeError(
		w,
		r,
		http.StatusTooManyRequests,
		"login_locked",
		"Too many failed login attempts. Try again later.",
	)
}

func loginRetryAfterSeconds(
	duration time.Duration,
) int64 {
	if duration <= 0 {
		return 1
	}

	seconds := int64(
		(duration + time.Second - 1) /
			time.Second,
	)

	if seconds < 1 {
		return 1
	}

	return seconds
}

func (handler *Handler) logLoginProtectionClearFailure(
	r *http.Request,
) {
	if handler == nil || handler.logger == nil {
		return
	}

	handler.logger.Warnw(
		"failed to clear successful-login protection state",
		"request_id",
		chimiddleware.GetReqID(r.Context()),
	)
}
