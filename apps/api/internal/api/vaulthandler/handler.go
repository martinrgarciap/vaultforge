package vaulthandler

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/request"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
	"go.uber.org/zap"
)

const maxVaultRequestBodyBytes int64 = 4 * 1024

type VaultService interface {
	Create(
		ctx context.Context,
		input vault.CreateInput,
	) (vault.Vault, error)

	List(
		ctx context.Context,
		ownerID string,
	) ([]vault.Vault, error)

	Get(
		ctx context.Context,
		ownerID string,
		vaultID string,
	) (vault.Vault, error)

	Rename(
		ctx context.Context,
		input vault.RenameInput,
	) (vault.Vault, error)

	Delete(
		ctx context.Context,
		input vault.DeleteInput,
	) error
}

type Handler struct {
	vaultService VaultService
	logger       *zap.SugaredLogger
}

type createVaultRequest struct {
	Name string `json:"name"`
}

type renameVaultRequest struct {
	Name string `json:"name"`
}

type vaultResponse struct {
	Vault vaultResource `json:"vault"`
}

type vaultListResponse struct {
	Vaults []vaultResource `json:"vaults"`
}

type vaultResource struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CryptoVersion *int16    `json:"cryptoVersion,omitempty"`
	KDFVersion    *int16    `json:"kdfVersion,omitempty"`
	Salt          *string   `json:"salt,omitempty"`
	WrappedKey    *string   `json:"wrappedKey,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func New(
	vaultService VaultService,
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		vaultService: vaultService,
		logger:       logger,
	}
}

func (handler *Handler) Create(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	var requestBody createVaultRequest

	err := request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxVaultRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)
		return
	}

	createdVault, err := handler.vaultService.Create(
		r.Context(),
		vault.CreateInput{
			OwnerID: principal.UserID,
			Name:    requestBody.Name,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	err = response.WriteJSON(
		w,
		http.StatusCreated,
		vaultResponse{
			Vault: newVaultResource(
				createdVault,
			),
		},
	)
	if err != nil {
		handler.logResponseFailure(r)
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

	storedVaults, err := handler.vaultService.List(
		r.Context(),
		principal.UserID,
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	vaults := make(
		[]vaultResource,
		0,
		len(storedVaults),
	)

	for _, storedVault := range storedVaults {
		vaults = append(
			vaults,
			newVaultResource(storedVault),
		)
	}

	err = response.WriteJSON(
		w,
		http.StatusOK,
		vaultListResponse{
			Vaults: vaults,
		},
	)
	if err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) Get(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	storedVault, err := handler.vaultService.Get(
		r.Context(),
		principal.UserID,
		chi.URLParam(r, "vaultID"),
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	err = response.WriteJSON(
		w,
		http.StatusOK,
		vaultResponse{
			Vault: newVaultResource(
				storedVault,
			),
		},
	)
	if err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) Rename(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	var requestBody renameVaultRequest

	err := request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxVaultRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)
		return
	}

	renamedVault, err := handler.vaultService.Rename(
		r.Context(),
		vault.RenameInput{
			OwnerID: principal.UserID,
			VaultID: chi.URLParam(r, "vaultID"),
			Name:    requestBody.Name,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	err = response.WriteJSON(
		w,
		http.StatusOK,
		vaultResponse{
			Vault: newVaultResource(
				renamedVault,
			),
		},
	)
	if err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) Delete(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	err := handler.vaultService.Delete(
		r.Context(),
		vault.DeleteInput{
			OwnerID:       principal.UserID,
			VaultID:       chi.URLParam(r, "vaultID"),
			CorrelationID: chimiddleware.GetReqID(r.Context()),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func newVaultResource(
	storedVault vault.Vault,
) vaultResource {
	return vaultResource{
		ID:            storedVault.ID,
		Name:          storedVault.Name,
		CryptoVersion: storedVault.CryptoVersion,
		KDFVersion:    storedVault.KDFVersion,
		Salt:          encodeOptionalVaultCryptoBytes(storedVault.Salt),
		WrappedKey:    encodeOptionalVaultCryptoBytes(storedVault.WrappedKey),
		CreatedAt:     storedVault.CreatedAt,
		UpdatedAt:     storedVault.UpdatedAt,
	}
}

func encodeOptionalVaultCryptoBytes(value []byte) *string {
	if len(value) == 0 {
		return nil
	}

	encoded := base64.StdEncoding.EncodeToString(value)

	return &encoded
}

func (handler *Handler) principal(
	w http.ResponseWriter,
	r *http.Request,
) (session.Principal, bool) {
	if handler == nil ||
		handler.vaultService == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"vault_unavailable",
			"Vault operations are temporarily unavailable.",
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

func (handler *Handler) writeServiceError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(
		err,
		vault.ErrVaultNameInvalidUTF8,
	),
		errors.Is(
			err,
			vault.ErrVaultNameEmpty,
		),
		errors.Is(
			err,
			vault.ErrVaultNameTooLong,
		),
		errors.Is(
			err,
			vault.ErrVaultNameContainsControlCharacter,
		):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_vault_name",
			"Vault name must be valid Unicode, contain no control characters, and be between 1 and 128 characters.",
		)

	case errors.Is(
		err,
		vault.ErrVaultNotFound,
	):
		handler.writeError(
			w,
			r,
			http.StatusNotFound,
			"vault_not_found",
			"The vault was not found.",
		)

	case errors.Is(
		err,
		vault.ErrOwnerInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusUnauthorized,
			"unauthorized",
			"A valid access token is required.",
		)

	case errors.Is(
		err,
		vault.ErrVaultCryptoMetadataInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_vault_crypto_metadata",
			"The vault crypto metadata is invalid.",
		)

	case errors.Is(
		err,
		vault.ErrVaultCryptoMetadataAlreadyInitialized,
	):
		handler.writeError(
			w,
			r,
			http.StatusConflict,
			"vault_crypto_metadata_already_initialized",
			"The vault crypto metadata has already been initialized.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(
			err,
			vault.ErrVaultUnavailable,
		):
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"vault_unavailable",
			"Vault operations are temporarily unavailable.",
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

	err := response.WriteError(
		w,
		status,
		code,
		message,
		requestID,
	)
	if err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) logResponseFailure(
	r *http.Request,
) {
	if handler == nil ||
		handler.logger == nil {
		return
	}

	handler.logger.Errorw(
		"failed to write vault response",
		"request_id",
		chimiddleware.GetReqID(
			r.Context(),
		),
	)
}
