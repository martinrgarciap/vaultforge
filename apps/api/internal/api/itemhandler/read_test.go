package itemhandler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const itemHandlerReadSecondItemID = "00000000-0000-0000-0000-000000002105"

func TestHandlerListsVaultItems(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(
		2026,
		time.June,
		23,
		21,
		0,
		0,
		0,
		time.UTC,
	)
	deletedAt := updatedAt

	cursor, err := encodeItemCursor(
		vault.ItemCursor{
			UpdatedAt: updatedAt,
			ID:        itemHandlerCreateTestItemID,
		},
	)
	if err != nil {
		t.Fatalf("encode test cursor: %v", err)
	}

	service := &itemHandlerTestService{
		listResult: vault.ItemPage{
			Items: []vault.Item{
				{
					ID:      itemHandlerCreateTestItemID,
					VaultID: itemHandlerCreateTestVaultID,
					Type:    vault.ItemTypeSecureNote,
					Payload: json.RawMessage(
						`{"value":"synthetic"}`,
					),
					Version:   2,
					CreatedAt: updatedAt.Add(-time.Hour),
					UpdatedAt: updatedAt,
					DeletedAt: &deletedAt,
				},
			},
			NextCursor: &vault.ItemCursor{
				UpdatedAt: updatedAt,
				ID:        itemHandlerCreateTestItemID,
			},
		},
	}

	router := newItemHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items?state=deleted&limit=1&after="+
			url.QueryEscape(cursor),
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
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

	if service.listCalls != 1 {
		t.Fatalf(
			"ListItems() calls = %d, want 1",
			service.listCalls,
		)
	}

	if service.lastListInput.OwnerID !=
		itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			service.lastListInput.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if service.lastListInput.VaultID !=
		itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			service.lastListInput.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	options := service.lastListInput.Options

	if options.State != vault.ItemListStateDeleted {
		t.Fatalf(
			"state = %q, want %q",
			options.State,
			vault.ItemListStateDeleted,
		)
	}

	if options.Limit != 1 {
		t.Fatalf(
			"limit = %d, want 1",
			options.Limit,
		)
	}

	if options.After == nil {
		t.Fatal("list options omitted the cursor")
	}

	if options.After.ID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"cursor ID = %q, want %q",
			options.After.ID,
			itemHandlerCreateTestItemID,
		)
	}

	var body itemListResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode item list response: %v",
			err,
		)
	}

	if len(body.Items) != 1 {
		t.Fatalf(
			"item count = %d, want 1",
			len(body.Items),
		)
	}

	if body.NextCursor == "" {
		t.Fatal(
			"item list response omitted the next cursor",
		)
	}

	responseText := recorder.Body.String()

	if strings.Contains(
		responseText,
		itemHandlerTestOwnerID,
	) {
		t.Fatal("item list exposed the owner ID")
	}

	if strings.Contains(
		responseText,
		itemHandlerCreateTestVaultID,
	) {
		t.Fatal("item list exposed the vault ID")
	}
}

func TestHandlerListsEmptyItemCollection(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{
		listResult: vault.ItemPage{},
	}

	router := newItemHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusOK,
		)
	}

	if recorder.Body.String() != "{\"items\":[]}\n" {
		t.Fatalf(
			"body = %q, want empty item collection",
			recorder.Body.String(),
		)
	}

	if service.lastListInput.Options.State !=
		vault.ItemListStateActive {
		t.Fatalf(
			"default state = %q, want %q",
			service.lastListInput.Options.State,
			vault.ItemListStateActive,
		)
	}

	if service.lastListInput.Options.Limit !=
		vault.DefaultItemPageLimit {
		t.Fatalf(
			"default limit = %d, want %d",
			service.lastListInput.Options.Limit,
			vault.DefaultItemPageLimit,
		)
	}
}

func TestHandlerListRejectsInvalidQueries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		query    string
		wantCode string
	}{
		{
			name:     "unsupported parameter",
			query:    "?sort=name",
			wantCode: "invalid_query",
		},
		{
			name:     "repeated state",
			query:    "?state=active&state=deleted",
			wantCode: "invalid_query",
		},
		{
			name:     "invalid state",
			query:    "?state=all",
			wantCode: "invalid_item_state",
		},
		{
			name:     "invalid lower limit",
			query:    "?limit=0",
			wantCode: "invalid_page_limit",
		},
		{
			name:     "invalid upper limit",
			query:    "?limit=101",
			wantCode: "invalid_page_limit",
		},
		{
			name:     "invalid cursor",
			query:    "?after=not-a-cursor",
			wantCode: "invalid_item_cursor",
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			service := &itemHandlerTestService{}
			router := newItemHandlerTestRouter(service)

			request := httptest.NewRequest(
				http.MethodGet,
				"/v1/vaults/"+
					itemHandlerCreateTestVaultID+
					"/items"+
					test.query,
				nil,
			)
			request.Header.Set(
				"Authorization",
				"Bearer synthetic-access-token",
			)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			assertItemHandlerTestError(
				t,
				recorder,
				http.StatusBadRequest,
				test.wantCode,
			)

			if service.listCalls != 0 {
				t.Fatal(
					"service was called for an invalid list query",
				)
			}
		})
	}
}

func TestHandlerGetsVaultItem(t *testing.T) {
	t.Parallel()

	updatedAt := time.Date(
		2026,
		time.June,
		23,
		22,
		0,
		0,
		0,
		time.UTC,
	)
	deletedAt := updatedAt

	service := &itemHandlerTestService{
		getResult: vault.Item{
			ID:      itemHandlerCreateTestItemID,
			VaultID: itemHandlerCreateTestVaultID,
			Type:    vault.ItemTypeAPIKey,
			Payload: json.RawMessage(
				`{"token":"synthetic-token"}`,
			),
			Version:   4,
			CreatedAt: updatedAt.Add(-time.Hour),
			UpdatedAt: updatedAt,
			DeletedAt: &deletedAt,
		},
	}

	router := newItemHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items/"+
			itemHandlerCreateTestItemID+
			"?state=deleted",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
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

	if service.getCalls != 1 {
		t.Fatalf(
			"GetItem() calls = %d, want 1",
			service.getCalls,
		)
	}

	if service.lastGetInput.OwnerID !=
		itemHandlerTestOwnerID {
		t.Fatalf(
			"owner ID = %q, want %q",
			service.lastGetInput.OwnerID,
			itemHandlerTestOwnerID,
		)
	}

	if service.lastGetInput.VaultID !=
		itemHandlerCreateTestVaultID {
		t.Fatalf(
			"vault ID = %q, want %q",
			service.lastGetInput.VaultID,
			itemHandlerCreateTestVaultID,
		)
	}

	if service.lastGetInput.ItemID !=
		itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			service.lastGetInput.ItemID,
			itemHandlerCreateTestItemID,
		)
	}

	if service.lastGetInput.State !=
		vault.ItemListStateDeleted {
		t.Fatalf(
			"state = %q, want %q",
			service.lastGetInput.State,
			vault.ItemListStateDeleted,
		)
	}

	if recorder.Header().Get("ETag") != `"4"` {
		t.Fatalf(
			"ETag = %q, want %q",
			recorder.Header().Get("ETag"),
			`"4"`,
		)
	}

	var body itemResponse

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode item response: %v",
			err,
		)
	}

	if body.Item.ID != itemHandlerCreateTestItemID {
		t.Fatalf(
			"item ID = %q, want %q",
			body.Item.ID,
			itemHandlerCreateTestItemID,
		)
	}

	if body.Item.Version != 4 {
		t.Fatalf(
			"item version = %d, want 4",
			body.Item.Version,
		)
	}
}

func TestHandlerGetMapsItemNotFoundSafely(
	t *testing.T,
) {
	t.Parallel()

	service := &itemHandlerTestService{
		getErr: vault.ErrItemNotFound,
	}
	router := newItemHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items/"+
			itemHandlerReadSecondItemID,
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusNotFound,
		"item_not_found",
	)
}

func TestHandlerGetRejectsInvalidQuery(t *testing.T) {
	t.Parallel()

	service := &itemHandlerTestService{}
	router := newItemHandlerTestRouter(service)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/"+
			itemHandlerCreateTestVaultID+
			"/items/"+
			itemHandlerCreateTestItemID+
			"?include=owner",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer synthetic-access-token",
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	assertItemHandlerTestError(
		t,
		recorder,
		http.StatusBadRequest,
		"invalid_query",
	)

	if service.getCalls != 0 {
		t.Fatal(
			"service was called for an invalid get query",
		)
	}
}
