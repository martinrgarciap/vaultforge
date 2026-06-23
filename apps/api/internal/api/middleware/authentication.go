package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

const maxBearerTokenBytes = 4 * 1024

type AccessTokenAuthenticator interface {
	AuthenticateAccessToken(
		ctx context.Context,
		tokenValue string,
	) (session.Principal, error)
}

type principalContextKey struct{}

func RequireAuthentication(
	authenticator AccessTokenAuthenticator,
	logger *zap.SugaredLogger,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			tokenValue, ok := bearerToken(r)
			if !ok {
				writeAuthenticationMiddlewareError(
					w,
					r,
					logger,
					http.StatusUnauthorized,
					"unauthorized",
					"A valid access token is required.",
				)

				return
			}

			if authenticator == nil {
				writeAuthenticationMiddlewareError(
					w,
					r,
					logger,
					http.StatusServiceUnavailable,
					"authentication_unavailable",
					"Authentication is temporarily unavailable.",
				)

				return
			}

			principal, err :=
				authenticator.AuthenticateAccessToken(
					r.Context(),
					tokenValue,
				)
			if err != nil {
				writeAccessAuthenticationError(
					w,
					r,
					logger,
					err,
				)

				return
			}

			requestContext := context.WithValue(
				r.Context(),
				principalContextKey{},
				principal,
			)

			next.ServeHTTP(
				w,
				r.WithContext(requestContext),
			)
		})
	}
}

func PrincipalFromContext(
	ctx context.Context,
) (session.Principal, bool) {
	if ctx == nil {
		return session.Principal{}, false
	}

	principal, ok := ctx.Value(
		principalContextKey{},
	).(session.Principal)

	if !ok ||
		principal.UserID == "" ||
		principal.SessionID == "" {
		return session.Principal{}, false
	}

	return principal, true
}

func bearerToken(
	r *http.Request,
) (string, bool) {
	if r == nil {
		return "", false
	}

	authorizationValues :=
		r.Header.Values("Authorization")

	if len(authorizationValues) != 1 {
		return "", false
	}

	authorization := authorizationValues[0]

	scheme, tokenValue, found :=
		strings.Cut(authorization, " ")
	if !found ||
		!strings.EqualFold(scheme, "Bearer") ||
		tokenValue == "" ||
		len(tokenValue) > maxBearerTokenBytes ||
		strings.ContainsAny(
			tokenValue,
			" \t\r\n",
		) {
		return "", false
	}

	return tokenValue, true
}

func writeAccessAuthenticationError(
	w http.ResponseWriter,
	r *http.Request,
	logger *zap.SugaredLogger,
	err error,
) {
	switch {
	case errors.Is(
		err,
		session.ErrAccessTokenInvalid,
	):
		writeAuthenticationMiddlewareError(
			w,
			r,
			logger,
			http.StatusUnauthorized,
			"unauthorized",
			"A valid access token is required.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(
			err,
			session.ErrSessionUnavailable,
		):
		writeAuthenticationMiddlewareError(
			w,
			r,
			logger,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

	default:
		writeAuthenticationMiddlewareError(
			w,
			r,
			logger,
			http.StatusInternalServerError,
			"internal_error",
			"An unexpected error occurred.",
		)
	}
}

func writeAuthenticationMiddlewareError(
	w http.ResponseWriter,
	r *http.Request,
	logger *zap.SugaredLogger,
	status int,
	code string,
	message string,
) {
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
	if err == nil || logger == nil {
		return
	}

	logger.Errorw(
		"failed to write authentication middleware response",
		"request_id",
		requestID,
	)
}
