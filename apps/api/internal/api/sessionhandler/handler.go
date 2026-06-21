package sessionhandler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"go.uber.org/zap"
)

type SessionService interface {
	ListSessions(
		ctx context.Context,
		principal session.Principal,
	) ([]session.SessionSummary, error)

	LogoutCurrent(
		ctx context.Context,
		principal session.Principal,
	) error

	RevokeSession(
		ctx context.Context,
		principal session.Principal,
		sessionID string,
	) error

	LogoutAll(
		ctx context.Context,
		principal session.Principal,
	) error
}

type Handler struct {
	sessionService SessionService
	logger         *zap.SugaredLogger
}

type sessionResponse struct {
	ID        string    `json:"id"`
	UserAgent string    `json:"userAgent"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	Current   bool      `json:"current"`
}

type listSessionsResponse struct {
	Sessions []sessionResponse `json:"sessions"`
}

func New(
	sessionService SessionService,
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		sessionService: sessionService,
		logger:         logger,
	}
}

func (handler *Handler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	summaries, err := handler.sessionService.ListSessions(
		r.Context(),
		principal,
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	sessions := make(
		[]sessionResponse,
		0,
		len(summaries),
	)

	for _, summary := range summaries {
		sessions = append(
			sessions,
			sessionResponse{
				ID:        summary.ID,
				UserAgent: summary.UserAgent,
				CreatedAt: summary.CreatedAt,
				ExpiresAt: summary.ExpiresAt,
				Current:   summary.Current,
			},
		)
	}

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		listSessionsResponse{
			Sessions: sessions,
		},
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) LogoutCurrent(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	err := handler.sessionService.LogoutCurrent(
		r.Context(),
		principal,
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) Revoke(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	err := handler.sessionService.RevokeSession(
		r.Context(),
		principal,
		chi.URLParam(r, "sessionID"),
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) LogoutAll(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	err := handler.sessionService.LogoutAll(
		r.Context(),
		principal,
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (handler *Handler) principal(
	w http.ResponseWriter,
	r *http.Request,
) (session.Principal, bool) {
	if handler == nil ||
		handler.sessionService == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"authentication_unavailable",
			"Authentication is temporarily unavailable.",
		)

		return session.Principal{}, false
	}

	principal, ok := appmiddleware.PrincipalFromContext(
		r.Context(),
	)
	if !ok {
		handler.writeError(
			w,
			r,
			http.StatusUnauthorized,
			"unauthorized",
			"A valid access token is required.",
		)

		return session.Principal{}, false
	}

	return principal, true
}

func (handler *Handler) writeServiceError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, session.ErrSessionNotFound):
		handler.writeError(
			w,
			r,
			http.StatusNotFound,
			"session_not_found",
			"The session was not found.",
		)

	case errors.Is(err, session.ErrPrincipalInvalid):
		handler.writeError(
			w,
			r,
			http.StatusUnauthorized,
			"unauthorized",
			"A valid access token is required.",
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

func (handler *Handler) writeError(
	w http.ResponseWriter,
	r *http.Request,
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
		"failed to write session response",
		"request_id",
		chimiddleware.GetReqID(r.Context()),
	)
}
