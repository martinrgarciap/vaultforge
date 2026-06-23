package authhandler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/request"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

const maxAuthRequestBodyBytes int64 = 64 * 1024

type RegistrationService interface {
	Register(
		ctx context.Context,
		input auth.RegisterInput,
	) (auth.Account, error)
}

type SessionService interface {
	Login(
		ctx context.Context,
		input session.LoginInput,
	) (session.LoginResult, error)

	Refresh(
		ctx context.Context,
		input session.RefreshInput,
	) (session.RefreshResult, error)
}

type Handler struct {
	registrationService RegistrationService
	sessionService      SessionService
	loginProtector      LoginProtector
	sessionCookies      *sessioncookie.Manager
	logger              *zap.SugaredLogger
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type accountResponse struct {
	User auth.Account `json:"user"`
}

type loginResponse struct {
	User                  auth.Account `json:"user"`
	TokenType             string       `json:"tokenType"`
	AccessToken           string       `json:"accessToken"`
	AccessTokenExpiresAt  time.Time    `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt time.Time    `json:"refreshTokenExpiresAt"`
}

type refreshResponse struct {
	TokenType             string    `json:"tokenType"`
	AccessToken           string    `json:"accessToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

func New(
	registrationService RegistrationService,
	sessionService SessionService,
	loginProtector LoginProtector,
	sessionCookies *sessioncookie.Manager,
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		registrationService: registrationService,
		sessionService:      sessionService,
		loginProtector:      loginProtector,
		sessionCookies:      sessionCookies,
		logger:              logger,
	}
}

func (handler *Handler) Register(
	w http.ResponseWriter,
	r *http.Request,
) {
	if handler == nil ||
		handler.registrationService == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

		return
	}

	var requestBody credentialsRequest

	err := request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxAuthRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)

		return
	}

	account, err :=
		handler.registrationService.Register(
			r.Context(),
			auth.RegisterInput{
				Email:    requestBody.Email,
				Password: requestBody.Password,
			},
		)
	if err != nil {
		handler.writeRegisterError(w, r, err)

		return
	}

	if err := response.WriteJSON(
		w,
		http.StatusCreated,
		accountResponse{
			User: account,
		},
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) Login(
	w http.ResponseWriter,
	r *http.Request,
) {
	setNoStore(w)

	if handler == nil ||
		handler.sessionService == nil ||
		handler.loginProtector == nil ||
		handler.sessionCookies == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

		return
	}

	var requestBody credentialsRequest

	err := request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxAuthRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)

		return
	}

	protectionIdentity, ok := loginProtectionIdentity(
		r,
		requestBody.Email,
	)
	if !ok {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

		return
	}

	lockout, err := handler.loginProtector.Check(
		r.Context(),
		protectionIdentity...,
	)
	if err != nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

		return
	}

	if lockout.Locked {
		handler.writeLoginLockout(
			w,
			r,
			lockout.RetryAfter,
		)

		return
	}

	csrfToken, err :=
		handler.sessionCookies.GenerateCSRFToken(
			r.Context(),
		)
	if err != nil {
		handler.writeCookieTransportError(w, r)

		return
	}

	result, err := handler.sessionService.Login(
		r.Context(),
		session.LoginInput{
			Email:     requestBody.Email,
			Password:  requestBody.Password,
			UserAgent: r.UserAgent(),
		},
	)
	if err != nil {
		if errors.Is(
			err,
			auth.ErrInvalidCredentials,
		) {
			lockout, protectionErr :=
				handler.loginProtector.RecordFailure(
					r.Context(),
					protectionIdentity...,
				)

			if protectionErr != nil {
				handler.writeError(
					w,
					r,
					http.StatusServiceUnavailable,
					"authentication_unavailable",
					"Authentication is temporarily unavailable.",
				)

				return
			}

			if lockout.Locked {
				handler.writeLoginLockout(
					w,
					r,
					lockout.RetryAfter,
				)

				return
			}
		}

		handler.writeLoginError(w, r, err)

		return
	}

	if err := handler.loginProtector.Clear(
		r.Context(),
		protectionIdentity...,
	); err != nil {
		handler.logLoginProtectionClearFailure(r)
	}

	if err := handler.sessionCookies.Set(
		w,
		result.RefreshToken.Value(),
		csrfToken,
		result.RefreshTokenExpiresAt,
	); err != nil {
		handler.writeCookieTransportError(w, r)

		return
	}

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		loginResponse{
			User:                  result.Account,
			TokenType:             "Bearer",
			AccessToken:           result.AccessToken.Value(),
			AccessTokenExpiresAt:  result.AccessToken.ExpiresAt(),
			RefreshTokenExpiresAt: result.RefreshTokenExpiresAt,
		},
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) Refresh(
	w http.ResponseWriter,
	r *http.Request,
) {
	setNoStore(w)

	if handler == nil || handler.sessionService == nil || handler.sessionCookies == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

		return
	}

	if err := request.RequireEmptyBody(w, r); err != nil {
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_request",
			"The refresh request must not contain a body.",
		)

		return
	}

	refreshToken, err := handler.sessionCookies.RefreshToken(r)
	if err != nil {
		handler.sessionCookies.Clear(w)
		handler.writeError(
			w,
			r,
			http.StatusUnauthorized,
			"invalid_refresh_token",
			"The refresh token is invalid or expired.",
		)
		return
	}

	if err := handler.sessionCookies.ValidateCSRF(r); err != nil {
		handler.writeError(
			w,
			r,
			http.StatusForbidden,
			"csrf_validation_failed",
			"The CSRF token is missing or invalid.",
		)
		return
	}

	csrfToken, err := handler.sessionCookies.GenerateCSRFToken(r.Context())
	if err != nil {
		handler.writeCookieTransportError(w, r)
		return
	}

	result, err := handler.sessionService.Refresh(
		r.Context(),
		session.RefreshInput{RefreshToken: refreshToken},
	)
	if err != nil {
		handler.writeRefreshError(w, r, err)
		return
	}

	if err := handler.sessionCookies.Set(
		w,
		result.RefreshToken.Value(),
		csrfToken,
		result.RefreshTokenExpiresAt,
	); err != nil {
		handler.writeCookieTransportError(w, r)
		return
	}

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		refreshResponse{
			TokenType:             "Bearer",
			AccessToken:           result.AccessToken.Value(),
			AccessTokenExpiresAt:  result.AccessToken.ExpiresAt(),
			RefreshTokenExpiresAt: result.RefreshTokenExpiresAt,
		},
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) writeDecodeError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(
		err,
		request.ErrUnsupportedContentType,
	):
		handler.writeError(
			w,
			r,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json.",
		)

	case errors.Is(
		err,
		request.ErrBodyTooLarge,
	):
		handler.writeError(
			w,
			r,
			http.StatusRequestEntityTooLarge,
			"request_body_too_large",
			"The request body is too large.",
		)

	default:
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_request",
			"The request body must contain one valid JSON object with only supported fields.",
		)
	}
}

func (handler *Handler) writeRegisterError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, auth.ErrEmailInvalid):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_email",
			"A valid email address is required.",
		)

	case errors.Is(err, auth.ErrPasswordInvalidUTF8),
		errors.Is(err, auth.ErrPasswordTooShort),
		errors.Is(err, auth.ErrPasswordTooLong):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_password",
			"Password must be valid Unicode and between 15 and 128 characters.",
		)

	case errors.Is(err, auth.ErrEmailUnavailable):
		handler.writeError(
			w,
			r,
			http.StatusConflict,
			"email_unavailable",
			"An account cannot be created with that email address.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(
			err,
			auth.ErrAuthenticationUnavailable,
		):
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

	default:
		handler.writeError(
			w,
			r,
			http.StatusInternalServerError,
			"internal_error",
			"An unexpected error occurred.",
		)
	}
}

func (handler *Handler) writeLoginError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		handler.writeError(
			w,
			r,
			http.StatusUnauthorized,
			"invalid_credentials",
			"The email or password is incorrect.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, session.ErrSessionUnavailable),
		errors.Is(err, auth.ErrAuthenticationUnavailable):
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

	default:
		handler.writeError(
			w,
			r,
			http.StatusInternalServerError,
			"internal_error",
			"An unexpected error occurred.",
		)
	}
}

func (handler *Handler) writeRefreshError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(
		err,
		session.ErrRefreshTokenInvalid,
	):
		handler.sessionCookies.Clear(w)
		handler.writeError(
			w,
			r,
			http.StatusUnauthorized,
			"invalid_refresh_token",
			"The refresh token is invalid or expired.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, session.ErrSessionUnavailable):
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

	default:
		handler.writeError(
			w,
			r,
			http.StatusInternalServerError,
			"internal_error",
			"An unexpected error occurred.",
		)
	}
}

func (handler *Handler) writeCookieTransportError(
	w http.ResponseWriter,
	r *http.Request,
) {
	handler.writeError(
		w,
		r,
		http.StatusServiceUnavailable,
		"authentication_unavailable",
		"Authentication is temporarily unavailable.",
	)
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func (handler *Handler) writeError(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	code string,
	message string,
) {
	requestID := middleware.GetReqID(
		r.Context(),
	)

	if err := response.WriteError(
		w,
		status,
		code,
		message,
		requestID,
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) logResponseFailure(
	r *http.Request,
) {
	if handler == nil || handler.logger == nil {
		return
	}

	handler.logger.Errorw(
		"failed to write authentication response",
		"request_id",
		middleware.GetReqID(r.Context()),
	)
}
