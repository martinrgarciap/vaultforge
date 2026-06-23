package middleware

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/ratelimit"
	"go.uber.org/zap"
)

type RequestLimiter interface {
	Allow(
		ctx context.Context,
		scope string,
		policy ratelimit.Policy,
		identityParts ...string,
	) (ratelimit.Decision, error)
}

func RateLimitByPeerIP(
	limiter RequestLimiter,
	scope string,
	policy ratelimit.Policy,
	unavailableCode string,
	unavailableMessage string,
	logger *zap.SugaredLogger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if limiter == nil {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusServiceUnavailable,
					unavailableCode,
					unavailableMessage,
					0,
				)

				return
			}

			clientIP, ok := PeerIP(r)
			if !ok {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusServiceUnavailable,
					unavailableCode,
					unavailableMessage,
					0,
				)

				return
			}

			decision, err := limiter.Allow(
				r.Context(),
				scope,
				policy,
				clientIP,
			)
			if err != nil {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusServiceUnavailable,
					unavailableCode,
					unavailableMessage,
					0,
				)

				return
			}

			if !decision.Allowed {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusTooManyRequests,
					"rate_limit_exceeded",
					"Too many requests. Try again later.",
					decision.RetryAfter,
				)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func RateLimitByPrincipal(
	limiter RequestLimiter,
	scope string,
	policy ratelimit.Policy,
	unavailableCode string,
	unavailableMessage string,
	logger *zap.SugaredLogger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			principal, ok := PrincipalFromContext(
				r.Context(),
			)
			if !ok {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusUnauthorized,
					"unauthorized",
					"A valid access token is required.",
					0,
				)

				return
			}

			if limiter == nil {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusServiceUnavailable,
					unavailableCode,
					unavailableMessage,
					0,
				)

				return
			}

			decision, err := limiter.Allow(
				r.Context(),
				scope,
				policy,
				principal.UserID,
			)
			if err != nil {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusServiceUnavailable,
					unavailableCode,
					unavailableMessage,
					0,
				)

				return
			}

			if !decision.Allowed {
				writeRateLimitError(
					w,
					r,
					logger,
					scope,
					http.StatusTooManyRequests,
					"rate_limit_exceeded",
					"Too many requests. Try again later.",
					decision.RetryAfter,
				)

				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func PeerIP(
	r *http.Request,
) (string, bool) {
	if r == nil {
		return "", false
	}

	remoteAddress := strings.TrimSpace(
		r.RemoteAddr,
	)
	if remoteAddress == "" {
		return "", false
	}

	host := remoteAddress

	if splitHost, _, err := net.SplitHostPort(
		remoteAddress,
	); err == nil {
		host = splitHost
	} else {
		host = strings.TrimPrefix(host, "[")
		host = strings.TrimSuffix(host, "]")
	}

	address, err := netip.ParseAddr(host)
	if err != nil {
		return "", false
	}

	return address.Unmap().String(), true
}

func writeRateLimitError(
	w http.ResponseWriter,
	r *http.Request,
	logger *zap.SugaredLogger,
	scope string,
	status int,
	code string,
	message string,
	retryAfter time.Duration,
) {
	if status == http.StatusTooManyRequests {
		w.Header().Set(
			"Retry-After",
			strconv.FormatInt(
				retryAfterSeconds(retryAfter),
				10,
			),
		)
	}

	if status == http.StatusUnauthorized {
		w.Header().Set(
			"WWW-Authenticate",
			"Bearer",
		)
	}

	requestID := chimiddleware.GetReqID(
		r.Context(),
	)

	err := response.WriteError(
		w,
		status,
		code,
		message,
		requestID,
	)
	if err != nil && logger != nil {
		logger.Errorw(
			"failed to write rate-limit response",
			"request_id",
			requestID,
		)
	}

	if logger == nil {
		return
	}

	logger.Warnw(
		"request rejected by rate-limit enforcement",
		"request_id",
		requestID,
		"scope",
		scope,
		"status",
		status,
	)
}

func retryAfterSeconds(
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
