package passwordhandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/request"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/passwordclient"
	"go.uber.org/zap"
)

const maxPasswordRequestBodyBytes int64 = 4 * 1024

type Service interface {
	Generate(
		ctx context.Context,
		input passwordclient.GenerateInput,
	) (passwordclient.GenerateResult, error)

	CheckStrength(
		ctx context.Context,
		input passwordclient.CheckStrengthInput,
	) (passwordclient.CheckStrengthResult, error)
}

type Handler struct {
	service Service
	logger  *zap.SugaredLogger
}

type generateRequest struct {
	Length           uint32 `json:"length"`
	IncludeUppercase bool   `json:"includeUppercase"`
	IncludeLowercase bool   `json:"includeLowercase"`
	IncludeDigits    bool   `json:"includeDigits"`
	IncludeSymbols   bool   `json:"includeSymbols"`
	ExcludeChars     string `json:"excludeChars"`
}

type generateResponse struct {
	Password    string  `json:"password"`
	EntropyBits float64 `json:"entropyBits"`
}

type strengthRequest struct {
	Password string `json:"password"`
}

type strengthResponse struct {
	Score             uint32   `json:"score"`
	Label             string   `json:"label"`
	EntropyBits       float64  `json:"entropyBits"`
	CrackTimeEstimate string   `json:"crackTimeEstimate"`
	Suggestions       []string `json:"suggestions"`
}

func New(
	service Service,
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		service: service,
		logger:  logger,
	}
}

func (handler *Handler) Generate(
	w http.ResponseWriter,
	r *http.Request,
) {
	setNoStore(w)

	if handler == nil || handler.service == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"password_tools_unavailable",
			"Password tools are temporarily unavailable.",
		)

		return
	}

	var requestBody generateRequest

	err := request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxPasswordRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)

		return
	}

	result, err := handler.service.Generate(
		r.Context(),
		passwordclient.GenerateInput{
			Length:           requestBody.Length,
			IncludeUppercase: requestBody.IncludeUppercase,
			IncludeLowercase: requestBody.IncludeLowercase,
			IncludeDigits:    requestBody.IncludeDigits,
			IncludeSymbols:   requestBody.IncludeSymbols,
			ExcludeChars:     requestBody.ExcludeChars,
		},
	)
	if err != nil {
		handler.writePasswordServiceError(w, r, err)

		return
	}

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		generateResponse{
			Password:    result.Password,
			EntropyBits: result.EntropyBits,
		},
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) CheckStrength(
	w http.ResponseWriter,
	r *http.Request,
) {
	setNoStore(w)

	if handler == nil || handler.service == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"password_tools_unavailable",
			"Password tools are temporarily unavailable.",
		)

		return
	}

	var requestBody strengthRequest

	err := request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxPasswordRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)

		return
	}

	result, err := handler.service.CheckStrength(
		r.Context(),
		passwordclient.CheckStrengthInput{
			Password: requestBody.Password,
		},
	)
	if err != nil {
		handler.writePasswordServiceError(w, r, err)

		return
	}

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		strengthResponse{
			Score:             result.Score,
			Label:             result.Label,
			EntropyBits:       result.EntropyBits,
			CrackTimeEstimate: result.CrackTimeEstimate,
			Suggestions:       result.Suggestions,
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
	case errors.Is(err, request.ErrUnsupportedContentType):
		handler.writeError(
			w,
			r,
			http.StatusUnsupportedMediaType,
			"unsupported_media_type",
			"Content-Type must be application/json.",
		)

	case errors.Is(err, request.ErrBodyTooLarge):
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

func (handler *Handler) writePasswordServiceError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(err, passwordclient.ErrPasswordRequestInvalid):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_password_request",
			"The password request is invalid.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(err, passwordclient.ErrPasswordServiceUnavailable),
		errors.Is(err, passwordclient.ErrPasswordResponseInvalid):
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"password_tools_unavailable",
			"Password tools are temporarily unavailable.",
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
		"failed to write password tools response",
		"request_id",
		middleware.GetReqID(r.Context()),
	)
}

func setNoStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}
