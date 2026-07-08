package itemhandler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/request"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

type updateItemRequest struct {
	Type             vault.ItemType               `json:"type"`
	EncryptedPayload itemEncryptedPayloadResource `json:"encryptedPayload"`
}

func (handler *Handler) Update(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	expectedVersion, err := expectedItemVersion(r)
	if err != nil {
		handler.writeIfMatchError(w, r, err)
		return
	}

	var requestBody updateItemRequest

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

	encryptedEnvelope, err := requiredEncryptedItemEnvelopeFromResource(
		requestBody.EncryptedPayload,
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	updatedItem, err := handler.itemService.UpdateItem(
		r.Context(),
		vault.UpdateItemInput{
			OwnerID:           principal.UserID,
			VaultID:           chi.URLParam(r, "vaultID"),
			ItemID:            chi.URLParam(r, "itemID"),
			Type:              requestBody.Type,
			EncryptedEnvelope: &encryptedEnvelope,
			ExpectedVersion:   expectedVersion,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	resourceBody, err := newItemResource(updatedItem)
	if err != nil {
		handler.writeError(
			w,
			r,
			http.StatusInternalServerError,
			"internal_error",
			"An unexpected error occurred.",
		)
		return
	}

	w.Header().Set(
		"ETag",
		itemETag(updatedItem.Version),
	)

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		itemResponse{
			Item: resourceBody,
		},
	); err != nil {
		handler.logResponseFailure(r)
	}
}

func (handler *Handler) writeIfMatchError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
	case errors.Is(
		err,
		errIfMatchRequired,
	):
		handler.writeError(
			w,
			r,
			http.StatusPreconditionRequired,
			"if_match_required",
			"An If-Match header containing the current item version is required.",
		)

	default:
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_if_match",
			"The If-Match header must contain one quoted positive item version.",
		)
	}
}
