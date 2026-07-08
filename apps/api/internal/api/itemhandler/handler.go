package itemhandler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	appmiddleware "github.com/martinrgarciap/vaultforge/apps/api/internal/api/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/request"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/session"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
	"go.uber.org/zap"
)

const maxItemRequestBodyBytes int64 = vault.MaxEncryptedItemBlobBase64Bytes + 4*1024

type ItemService interface {
	CreateItem(
		ctx context.Context,
		input vault.CreateItemInput,
	) (vault.Item, error)

	ListItems(
		ctx context.Context,
		input vault.ListItemsInput,
	) (vault.ItemPage, error)

	GetItem(
		ctx context.Context,
		input vault.GetItemInput,
	) (vault.Item, error)

	UpdateItem(
		ctx context.Context,
		input vault.UpdateItemInput,
	) (vault.Item, error)

	SoftDeleteItem(
		ctx context.Context,
		input vault.SoftDeleteItemInput,
	) (vault.Item, error)

	RestoreItem(
		ctx context.Context,
		input vault.RestoreItemInput,
	) (vault.Item, error)

	PermanentDeleteItem(
		ctx context.Context,
		input vault.PermanentDeleteItemInput,
	) error
}

type Handler struct {
	itemService ItemService
	logger      *zap.SugaredLogger
}

type createItemRequest struct {
	Type             vault.ItemType               `json:"type"`
	Payload          json.RawMessage              `json:"payload"`
	EncryptedPayload itemEncryptedPayloadResource `json:"encryptedPayload"`
}

func New(
	itemService ItemService,
	logger *zap.SugaredLogger,
) *Handler {
	return &Handler{
		itemService: itemService,
		logger:      logger,
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

	key, err := idempotencyKey(r)
	if err != nil {
		handler.writeIdempotencyHeaderError(w, r, err)
		return
	}

	var requestBody createItemRequest

	err = request.DecodeJSON(
		w,
		r,
		&requestBody,
		maxItemRequestBodyBytes,
	)
	if err != nil {
		handler.writeDecodeError(w, r, err)
		return
	}

	encryptedEnvelope, err := encryptedItemEnvelopePointerFromResource(
		requestBody.EncryptedPayload,
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	vaultID := chi.URLParam(r, "vaultID")

	createdItem, err := handler.itemService.CreateItem(
		r.Context(),
		vault.CreateItemInput{
			OwnerID:           principal.UserID,
			VaultID:           vaultID,
			Type:              requestBody.Type,
			Payload:           requestBody.Payload,
			EncryptedEnvelope: encryptedEnvelope,
			IdempotencyKey:    key,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	w.Header().Set(
		"ETag",
		itemETag(createdItem.Version),
	)
	w.Header().Set(
		"Location",
		"/v1/vaults/"+
			vaultID+
			"/items/"+
			createdItem.ID,
	)

	err = response.WriteJSON(
		w,
		http.StatusCreated,
		itemResponse{
			Item: newItemResource(createdItem),
		},
	)
	if err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) principal(
	w http.ResponseWriter,
	r *http.Request,
) (session.Principal, bool) {
	if handler == nil ||
		handler.itemService == nil {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"item_unavailable",
			"Vault item operations are temporarily unavailable.",
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

func (handler *Handler) writeIdempotencyHeaderError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(
		err,
		errIdempotencyKeyRequired,
	):
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"idempotency_key_required",
			"An Idempotency-Key header is required.",
		)

	default:
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_idempotency_key",
			"The Idempotency-Key header is invalid.",
		)
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

func (handler *Handler) writeServiceError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(
		err,
		vault.ErrItemTypeInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_item_type",
			"The item type is invalid.",
		)

	case errors.Is(
		err,
		vault.ErrItemPayloadTooLarge,
	):
		handler.writeError(
			w,
			r,
			http.StatusRequestEntityTooLarge,
			"item_payload_too_large",
			"The item payload is too large.",
		)
	case errors.Is(
		err,
		vault.ErrItemEncryptedPayloadTooLarge,
	):
		handler.writeError(
			w,
			r,
			http.StatusRequestEntityTooLarge,
			"item_encrypted_payload_too_large",
			"The encrypted item payload is too large.",
		)

	case errors.Is(
		err,
		vault.ErrItemPayloadEmpty,
	),
		errors.Is(
			err,
			vault.ErrItemPayloadInvalid,
		),
		errors.Is(
			err,
			vault.ErrItemPayloadNotObject,
		):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_item_payload",
			"The item payload must be one valid JSON object.",
		)

	case errors.Is(
		err,
		vault.ErrItemEncryptedPayloadEmpty,
	),
		errors.Is(
			err,
			vault.ErrItemEncryptedPayloadInvalid,
		):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_encrypted_item_payload",
			"The encrypted item payload is invalid.",
		)

	case errors.Is(
		err,
		vault.ErrItemIdempotencyKeyInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusUnprocessableEntity,
			"invalid_idempotency_key",
			"The Idempotency-Key header is invalid.",
		)

	case errors.Is(
		err,
		vault.ErrItemIdempotencyConflict,
	):
		handler.writeError(
			w,
			r,
			http.StatusConflict,
			"idempotency_conflict",
			"The Idempotency-Key was already used with different request data.",
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
		vault.ErrItemListStateInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_item_state",
			"Item state must be active or deleted.",
		)

	case errors.Is(
		err,
		vault.ErrItemPageLimitInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_page_limit",
			"Page limit must be between 1 and 100.",
		)

	case errors.Is(
		err,
		vault.ErrItemCursorInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_item_cursor",
			"The item cursor is invalid.",
		)

	case errors.Is(
		err,
		vault.ErrItemNotFound,
	):
		handler.writeError(
			w,
			r,
			http.StatusNotFound,
			"item_not_found",
			"The vault item was not found.",
		)

	case errors.Is(
		err,
		vault.ErrItemVersionInvalid,
	):
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_item_version",
			"The item version is invalid.",
		)

	case errors.Is(
		err,
		vault.ErrItemConflict,
	):
		handler.writeError(
			w,
			r,
			http.StatusPreconditionFailed,
			"item_version_conflict",
			"The item changed after the supplied version was retrieved.",
		)

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded),
		errors.Is(
			err,
			vault.ErrItemUnavailable,
		):
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"item_unavailable",
			"Vault item operations are temporarily unavailable.",
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
		"failed to write vault item response",
		"request_id",
		chimiddleware.GetReqID(
			r.Context(),
		),
	)
}
