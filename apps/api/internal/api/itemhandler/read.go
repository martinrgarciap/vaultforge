package itemhandler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func (handler *Handler) List(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	options, err := parseItemListOptions(r)
	if err != nil {
		handler.writeQueryError(w, r, err)
		return
	}

	page, err := handler.itemService.ListItems(
		r.Context(),
		vault.ListItemsInput{
			OwnerID: principal.UserID,
			VaultID: chi.URLParam(r, "vaultID"),
			Options: options,
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	responseBody, err := newItemListResponse(page)
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

	if err := response.WriteJSON(
		w,
		http.StatusOK,
		responseBody,
	); err != nil {
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

	state, err := parseItemState(r)
	if err != nil {
		handler.writeQueryError(w, r, err)
		return
	}

	storedItem, err := handler.itemService.GetItem(
		r.Context(),
		vault.GetItemInput{
			OwnerID: principal.UserID,
			VaultID: chi.URLParam(r, "vaultID"),
			ItemID:  chi.URLParam(r, "itemID"),
			State:   state,
		},
	)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	resourceBody, err := newItemResource(storedItem)
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
		itemETag(storedItem.Version),
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

func (handler *Handler) writeQueryError(
	w http.ResponseWriter,
	r *http.Request,
	err error,
) {
	switch {
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

	default:
		handler.writeError(
			w,
			r,
			http.StatusBadRequest,
			"invalid_query",
			"The request query contains unsupported or repeated parameters.",
		)
	}
}
