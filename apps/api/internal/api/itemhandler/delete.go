package itemhandler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (handler *Handler) SoftDelete(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	expectedVersion, err := expectedSoftDeleteItemVersion(r)
	if err != nil {
		handler.writeIfMatchError(w, r, err)
		return
	}

	deletedItem, err := handler.itemService.SoftDeleteItem(
		r.Context(),
		vault.SoftDeleteItemInput{
			OwnerID:         principal.UserID,
			VaultID:         chi.URLParam(r, "vaultID"),
			ItemID:          chi.URLParam(r, "itemID"),
			ExpectedVersion: expectedVersion,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	resourceBody, err := newItemResource(deletedItem)
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
		itemETag(deletedItem.Version),
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

func (handler *Handler) Restore(
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

	restoredItem, err := handler.itemService.RestoreItem(
		r.Context(),
		vault.RestoreItemInput{
			OwnerID:         principal.UserID,
			VaultID:         chi.URLParam(r, "vaultID"),
			ItemID:          chi.URLParam(r, "itemID"),
			ExpectedVersion: expectedVersion,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	resourceBody, err := newItemResource(restoredItem)
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
		itemETag(restoredItem.Version),
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

func (handler *Handler) PermanentDelete(
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

	err = handler.itemService.PermanentDeleteItem(
		r.Context(),
		vault.PermanentDeleteItemInput{
			OwnerID:         principal.UserID,
			VaultID:         chi.URLParam(r, "vaultID"),
			ItemID:          chi.URLParam(r, "itemID"),
			ExpectedVersion: expectedVersion,
			CorrelationID: chimiddleware.GetReqID(
				r.Context(),
			),
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
