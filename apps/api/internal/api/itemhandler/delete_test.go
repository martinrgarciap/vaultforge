package itemhandler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestHandlerSoftDeletesVaultItem(t *testing.T) {
	t.Parallel()

	deletedAt := time.Date(
		2026,
		time.June,
		24,
		0,
		0,
		0,
		123456000,
		time.UTC,
	)

	service := &itemHandlerTestService{
		softDeleteResult: vault.Item{
			ID:      itemHandlerCreateTestItemID,
			VaultID: itemHandlerCreateTestVaultID,
			Type:    vault.ItemTypeSecureNote,
			Payload: json.RawMessage(
				`{"value":"synthetic"}`,
			),
			Version:   2,
			CreatedAt: deletedAt.Add(-time.Hour),
			UpdatedAt: deletedAt,
			DeletedAt: &deletedAt,
		},
	}

	router := newItemHandlerTestRouter(service)

	request := newItemMutationTestRequest(
		http.MethodDelete,
		"",
		`"2"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}

	if service.softDeleteCalls != 1 {
		t.Fatalf(
			"SoftDeleteItem() calls = %d, want 1",
			service.softDeleteCalls,
		)
	}

	input := service.lastSoftDeleteInput

	if input.OwnerID != itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			input.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if input.VaultID != itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			input.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	if input.ItemID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			input.ItemID,
			itemHandlerCreateTestItemID,
		)
	}

	if input.ExpectedVersion != 2 {
		t.Fatalf(
			"expected version = %d, want 2",
			input.ExpectedVersion,
		)
	}

	if input.CorrelationID == "" {
		t.Fatal(
			"soft-delete input omitted the correlation ID",
		)
	}

	if recorder.Header().Get("ETag") != `"2"` {
		t.Fatalf(
			"ETag = %q, want %q",
			recorder.Header().Get("ETag"),
			`"2"`,
		)
	}

	var body itemResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode soft-delete response: %v",
			err,
		)
	}

	if body.Item.DeletedAt == nil {
		t.Fatal(
			"soft-delete response did not mark the item deleted",
		)
	}
}

func TestHandlerRestoresVaultItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(
		2026,
		time.June,
		24,
		1,
		0,
		0,
		123456000,
		time.UTC,
	)

	service := &itemHandlerTestService{
		restoreResult: vault.Item{
			ID:      itemHandlerCreateTestItemID,
			VaultID: itemHandlerCreateTestVaultID,
			Type:    vault.ItemTypeSecureNote,
			Payload: json.RawMessage(
				`{"value":"synthetic"}`,
			),
			Version:   3,
			CreatedAt: updatedAt.Add(-time.Hour),
			UpdatedAt: updatedAt,
		},
	}

	router := newItemHandlerTestRouter(service)

	request := newItemMutationTestRequest(
		http.MethodPost,
		"/restore",
		`"2"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusOK,
			recorder.Body.String(),
		)
	}

	if service.restoreCalls != 1 {
		t.Fatalf(
			"RestoreItem() calls = %d, want 1",
			service.restoreCalls,
		)
	}

	input := service.lastRestoreInput

	if input.OwnerID != itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			input.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if input.VaultID != itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			input.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	if input.ItemID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			input.ItemID,
			itemHandlerCreateTestItemID,
		)
	}

	if input.ExpectedVersion != 2 {
		t.Fatalf(
			"expected version = %d, want 2",
			input.ExpectedVersion,
		)
	}

	if input.CorrelationID == "" {
		t.Fatal(
			"restore input omitted the correlation ID",
		)
	}

	if recorder.Header().Get("ETag") != `"3"` {
		t.Fatalf(
			"ETag = %q, want %q",
			recorder.Header().Get("ETag"),
			`"3"`,
		)
	}

	var body itemResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode restore response: %v",
			err,
		)
	}

	if body.Item.DeletedAt != nil {
		t.Fatal(
			"restore response still marked the item deleted",
		)
	}

	if body.Item.Version != 3 {
		t.Fatalf(
			"restored version = %d, want 3",
			body.Item.Version,
		)
	}
}

func TestHandlerPermanentlyDeletesVaultItem(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := newItemMutationTestRequest(
		http.MethodDelete,
		"/permanent",
		`"3"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			http.StatusNoContent,
			recorder.Body.String(),
		)
	}

	if recorder.Body.Len() != 0 {
		t.Fatalf(
			"response body length = %d, want 0",
			recorder.Body.Len(),
		)
	}

	if service.permanentDeleteCalls != 1 {
		t.Fatalf(
			"PermanentDeleteItem() calls = %d, want 1",
			service.permanentDeleteCalls,
		)
	}

	input := service.lastPermanentDeleteInput

	if input.OwnerID != itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			input.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if input.VaultID != itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			input.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	if input.ItemID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			input.ItemID,
			itemHandlerCreateTestItemID,
		)
	}

	if input.ExpectedVersion != 3 {
		t.Fatalf(
			"expected version = %d, want 3",
			input.ExpectedVersion,
		)
	}

	if input.CorrelationID == "" {
		t.Fatal(
			"permanent-delete input omitted the correlation ID",
		)
	}
}

func TestHandlerItemMutationsRequireIfMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		pathSuffix string
		callCount  func(*itemHandlerTestService) int
	}{
		{
			name:       "soft delete",
			method:     http.MethodDelete,
			pathSuffix: "",
			callCount: func(
				service *itemHandlerTestService,
			) int {
				return service.softDeleteCalls
			},
		},
		{
			name:       "restore",
			method:     http.MethodPost,
			pathSuffix: "/restore",
			callCount: func(
				service *itemHandlerTestService,
			) int {
				return service.restoreCalls
			},
		},
		{
			name:       "permanent delete",
			method:     http.MethodDelete,
			pathSuffix: "/permanent",
			callCount: func(
				service *itemHandlerTestService,
			) int {
				return service.permanentDeleteCalls
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &itemHandlerTestService{}
			router := newItemHandlerTestRouter(service)

			request := newItemMutationTestRequest(
				test.method,
				test.pathSuffix,
				"",
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assertItemHandlerTestError(
				t,
				recorder,
				http.StatusPreconditionRequired,
				"if_match_required",
			)

			if test.callCount(service) != 0 {
				t.Fatal(
					"service was called without If-Match",
				)
			}
		})
	}
}

func TestHandlerItemMutationsMapConflicts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		pathSuffix string
		service    *itemHandlerTestService
	}{
		{
			name:       "soft delete",
			method:     http.MethodDelete,
			pathSuffix: "",
			service: &itemHandlerTestService{
				softDeleteErr: vault.ErrItemConflict,
			},
		},
		{
			name:       "restore",
			method:     http.MethodPost,
			pathSuffix: "/restore",
			service: &itemHandlerTestService{
				restoreErr: vault.ErrItemConflict,
			},
		},
		{
			name:       "permanent delete",
			method:     http.MethodDelete,
			pathSuffix: "/permanent",
			service: &itemHandlerTestService{
				permanentDeleteErr: vault.ErrItemConflict,
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := newItemHandlerTestRouter(
				test.service,
			)

			request := newItemMutationTestRequest(
				test.method,
				test.pathSuffix,
				`"1"`,
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assertItemHandlerTestError(
				t,
				recorder,
				http.StatusPreconditionFailed,
				"item_version_conflict",
			)
		})
	}
}

func TestHandlerItemMutationsMapNotFound(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		pathSuffix string
		service    *itemHandlerTestService
	}{
		{
			name:       "soft delete",
			method:     http.MethodDelete,
			pathSuffix: "",
			service: &itemHandlerTestService{
				softDeleteErr: vault.ErrItemNotFound,
			},
		},
		{
			name:       "restore",
			method:     http.MethodPost,
			pathSuffix: "/restore",
			service: &itemHandlerTestService{
				restoreErr: vault.ErrItemNotFound,
			},
		},
		{
			name:       "permanent delete",
			method:     http.MethodDelete,
			pathSuffix: "/permanent",
			service: &itemHandlerTestService{
				permanentDeleteErr: vault.ErrItemNotFound,
			},
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			router := newItemHandlerTestRouter(
				test.service,
			)

			request := newItemMutationTestRequest(
				test.method,
				test.pathSuffix,
				`"1"`,
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assertItemHandlerTestError(
				t,
				recorder,
				http.StatusNotFound,
				"item_not_found",
			)
		})
	}
}

func TestHandlerPermanentDeleteDoesNotExposeInternalError(
	t *testing.T,
) {
	t.Parallel()

	const internalMarker = "synthetic permanent-delete failure"

	service := &itemHandlerTestService{
		permanentDeleteErr: errors.New(
			internalMarker,
		),
	}

	router := newItemHandlerTestRouter(service)

	request := newItemMutationTestRequest(
		http.MethodDelete,
		"/permanent",
		`"1"`,
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusInternalServerError,
		"internal_error",
	)

	if strings.Contains(
		recorder.Body.String(),
		internalMarker,
	) {
		t.Fatal(
			"response exposed the internal service error",
		)
	}
}

func newItemMutationTestRequest(
	method string,
	pathSuffix string,
	ifMatch string,
) *http.Request {
	request := httptest.NewRequest(
		method,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items/"+
			itemHandlerCreateTestItemID+
			pathSuffix,
		nil,
	)

	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)
	request.Header.Set(
		"X-Request-ID",
		itemHandlerTestRequestID,
	)

	if ifMatch != "" {
		request.Header.Set(
			"If-Match",
			ifMatch,
		)
	}

	return request
}
