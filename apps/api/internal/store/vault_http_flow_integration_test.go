package store_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	vaultHTTPCreateRequestID = "vault-http-create-request"
	vaultHTTPRenameRequestID = "vault-http-rename-request"
)

type vaultHTTPTestUser struct {
	ID          string
	AccessToken string
}

type vaultHTTPResource struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	CryptoVersion *int16    `json:"cryptoVersion,omitempty"`
	KDFVersion    *int16    `json:"kdfVersion,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type vaultHTTPResponse struct {
	Vault vaultHTTPResource `json:"vault"`
}

type vaultHTTPListResponse struct {
	Vaults []vaultHTTPResource `json:"vaults"`
}

func TestVaultHTTPFlowAndOwnershipIntegration(t *testing.T) {
	app, databasePool, _ := newAuthenticationIntegrationApplication(t)
	router := app.Routes()

	owner := registerAndLoginVaultHTTPUser(t, router, "vault-owner@example.com")
	otherUser := registerAndLoginVaultHTTPUser(t, router, "vault-other@example.com")

	createRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodPost,
		"/v1/vaults",
		`{"name":"  Development  "}`,
		vaultHTTPCreateRequestID,
	)

	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRecorder.Code, http.StatusCreated)
	}

	var createResponse vaultHTTPResponse

	if err := json.NewDecoder(createRecorder.Body).Decode(&createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	createdVault := createResponse.Vault

	if createdVault.ID == "" {
		t.Fatal("create response did not include a vault ID")
	}

	if createdVault.Name != "Development" {
		t.Fatalf("created vault name = %q, want Development", createdVault.Name)
	}

	if createdVault.CreatedAt.IsZero() || createdVault.UpdatedAt.IsZero() {
		t.Fatal("create response did not include vault timestamps")
	}

	if strings.Contains(createRecorder.Body.String(), owner.ID) {
		t.Fatal("create response exposed the internal owner ID")
	}

	assertStoredVault(
		t,
		databasePool,
		createdVault.ID,
		owner.ID,
		"Development",
	)

	listOwnerRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodGet,
		"/v1/vaults",
		"",
		"",
	)

	if listOwnerRecorder.Code != http.StatusOK {
		t.Fatalf("owner list status = %d, want %d", listOwnerRecorder.Code, http.StatusOK)
	}

	var ownerList vaultHTTPListResponse

	if err := json.NewDecoder(listOwnerRecorder.Body).Decode(&ownerList); err != nil {
		t.Fatalf("decode owner vault list: %v", err)
	}

	if len(ownerList.Vaults) != 1 {
		t.Fatalf("owner vault count = %d, want 1", len(ownerList.Vaults))
	}

	if ownerList.Vaults[0].ID != createdVault.ID {
		t.Fatalf("listed vault ID = %q, want %q", ownerList.Vaults[0].ID, createdVault.ID)
	}

	listOtherRecorder := performVaultHTTPRequest(
		router,
		otherUser.AccessToken,
		http.MethodGet,
		"/v1/vaults",
		"",
		"",
	)

	if listOtherRecorder.Code != http.StatusOK {
		t.Fatalf("other-user list status = %d, want %d", listOtherRecorder.Code, http.StatusOK)
	}

	var otherList vaultHTTPListResponse

	if err := json.NewDecoder(listOtherRecorder.Body).Decode(&otherList); err != nil {
		t.Fatalf("decode other-user vault list: %v", err)
	}

	if otherList.Vaults == nil {
		t.Fatal("other-user empty vault collection was null")
	}

	if len(otherList.Vaults) != 0 {
		t.Fatalf("other-user vault count = %d, want 0", len(otherList.Vaults))
	}

	getOwnerRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodGet,
		"/v1/vaults/"+createdVault.ID,
		"",
		"",
	)

	if getOwnerRecorder.Code != http.StatusOK {
		t.Fatalf("owner get status = %d, want %d", getOwnerRecorder.Code, http.StatusOK)
	}

	getOtherRecorder := performVaultHTTPRequest(
		router,
		otherUser.AccessToken,
		http.MethodGet,
		"/v1/vaults/"+createdVault.ID,
		"",
		"",
	)

	assertVaultHTTPError(t, getOtherRecorder, http.StatusNotFound, "vault_not_found")

	renameOtherRecorder := performVaultHTTPRequest(
		router,
		otherUser.AccessToken,
		http.MethodPatch,
		"/v1/vaults/"+createdVault.ID,
		`{"name":"Unauthorized Rename"}`,
		"vault-http-cross-user-rename",
	)

	assertVaultHTTPError(t, renameOtherRecorder, http.StatusNotFound, "vault_not_found")

	assertStoredVault(
		t,
		databasePool,
		createdVault.ID,
		owner.ID,
		"Development",
	)

	deleteOtherRecorder := performVaultHTTPRequest(
		router,
		otherUser.AccessToken,
		http.MethodDelete,
		"/v1/vaults/"+createdVault.ID,
		"",
		"vault-http-cross-user-delete",
	)

	assertVaultHTTPError(t, deleteOtherRecorder, http.StatusNotFound, "vault_not_found")

	assertStoredVault(
		t,
		databasePool,
		createdVault.ID,
		owner.ID,
		"Development",
	)

	renameOwnerRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodPatch,
		"/v1/vaults/"+createdVault.ID,
		`{"name":"Renamed Vault"}`,
		vaultHTTPRenameRequestID,
	)

	if renameOwnerRecorder.Code != http.StatusOK {
		t.Fatalf("owner rename status = %d, want %d", renameOwnerRecorder.Code, http.StatusOK)
	}

	var renameResponse vaultHTTPResponse

	if err := json.NewDecoder(renameOwnerRecorder.Body).Decode(&renameResponse); err != nil {
		t.Fatalf("decode rename response: %v", err)
	}

	if renameResponse.Vault.Name != "Renamed Vault" {
		t.Fatalf("renamed vault name = %q, want Renamed Vault", renameResponse.Vault.Name)
	}

	if renameResponse.Vault.UpdatedAt.Before(createdVault.UpdatedAt) {
		t.Fatalf(
			"renamed updated time = %v, created updated time = %v",
			renameResponse.Vault.UpdatedAt,
			createdVault.UpdatedAt,
		)
	}

	assertStoredVault(
		t,
		databasePool,
		createdVault.ID,
		owner.ID,
		"Renamed Vault",
	)

	deleteOwnerRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodDelete,
		"/v1/vaults/"+createdVault.ID,
		"",
		"vault-http-owner-delete",
	)

	if deleteOwnerRecorder.Code != http.StatusNoContent {
		t.Fatalf(
			"owner delete status = %d, want %d",
			deleteOwnerRecorder.Code,
			http.StatusNoContent,
		)
	}

	if deleteOwnerRecorder.Body.Len() != 0 {
		t.Fatalf("delete response body length = %d, want 0", deleteOwnerRecorder.Body.Len())
	}

	getDeletedRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodGet,
		"/v1/vaults/"+createdVault.ID,
		"",
		"",
	)

	assertVaultHTTPError(t, getDeletedRecorder, http.StatusNotFound, "vault_not_found")
	assertVaultDeleted(t, databasePool, createdVault.ID)

	listDeletedRecorder := performVaultHTTPRequest(
		router,
		owner.AccessToken,
		http.MethodGet,
		"/v1/vaults",
		"",
		"",
	)

	if listDeletedRecorder.Code != http.StatusOK {
		t.Fatalf(
			"post-delete list status = %d, want %d",
			listDeletedRecorder.Code,
			http.StatusOK,
		)
	}

	var deletedList vaultHTTPListResponse

	if err := json.NewDecoder(listDeletedRecorder.Body).Decode(&deletedList); err != nil {
		t.Fatalf("decode post-delete vault list: %v", err)
	}

	if deletedList.Vaults == nil {
		t.Fatal("post-delete empty vault collection was null")
	}

	if len(deletedList.Vaults) != 0 {
		t.Fatalf("post-delete vault count = %d, want 0", len(deletedList.Vaults))
	}
}

func registerAndLoginVaultHTTPUser(
	t *testing.T,
	router http.Handler,
	email string,
) vaultHTTPTestUser {
	t.Helper()

	registerRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/register",
		email,
		authIntegrationPassword,
	)

	if registerRecorder.Code != http.StatusCreated {
		t.Fatalf("registration status = %d, want %d", registerRecorder.Code, http.StatusCreated)
	}

	var registerResponse authenticationAccountResponse

	if err := json.NewDecoder(registerRecorder.Body).Decode(&registerResponse); err != nil {
		t.Fatalf("decode vault HTTP registration response: %v", err)
	}

	loginRecorder := performAuthenticationRequest(
		t,
		router,
		"/v1/auth/login",
		email,
		authIntegrationPassword,
	)

	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", loginRecorder.Code, http.StatusOK)
	}

	var loginResponse authenticationLoginResponse

	if err := json.NewDecoder(loginRecorder.Body).Decode(&loginResponse); err != nil {
		t.Fatalf("decode vault HTTP login response: %v", err)
	}

	if loginResponse.AccessToken == "" {
		t.Fatal("vault HTTP login response did not include an access token")
	}

	return vaultHTTPTestUser{
		ID:          registerResponse.User.ID,
		AccessToken: loginResponse.AccessToken,
	}
}

func performVaultHTTPRequest(
	router http.Handler,
	accessToken string,
	method string,
	path string,
	body string,
	requestID string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+accessToken)

	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	if requestID != "" {
		request.Header.Set("X-Request-ID", requestID)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	return recorder
}

func assertVaultHTTPError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d", recorder.Code, wantStatus)
	}

	var response authenticationErrorResponse

	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decode vault HTTP error response: %v", err)
	}

	if response.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %q", response.Error.Code, wantCode)
	}

	if response.Error.Message == "" {
		t.Fatal("vault HTTP error response did not include a safe message")
	}

	if response.Error.RequestID == "" {
		t.Fatal("vault HTTP error response did not include a request ID")
	}
}

func assertStoredVault(
	t *testing.T,
	databasePool *pgxpool.Pool,
	vaultID string,
	wantOwnerID string,
	wantName string,
) {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelQuery()

	var (
		storedOwnerID string
		storedName    string
	)

	err := databasePool.QueryRow(
		queryContext,
		`
			SELECT
				owner_id::text,
				name
			FROM vaults
			WHERE id = $1::uuid
		`,
		vaultID,
	).Scan(&storedOwnerID, &storedName)
	if err != nil {
		t.Fatalf("read stored vault: %v", err)
	}

	if storedOwnerID != wantOwnerID {
		t.Fatalf("stored owner ID = %q, want %q", storedOwnerID, wantOwnerID)
	}

	if storedName != wantName {
		t.Fatalf("stored vault name = %q, want %q", storedName, wantName)
	}
}

func assertVaultDeleted(
	t *testing.T,
	databasePool *pgxpool.Pool,
	vaultID string,
) {
	t.Helper()

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		authIntegrationTimeout,
	)
	defer cancelQuery()

	var vaultCount int

	err := databasePool.QueryRow(
		queryContext,
		`
			SELECT count(*)
			FROM vaults
			WHERE id = $1::uuid
		`,
		vaultID,
	).Scan(&vaultCount)
	if err != nil {
		t.Fatalf("count deleted vault: %v", err)
	}

	if vaultCount != 0 {
		t.Fatalf("deleted vault count = %d, want 0", vaultCount)
	}
}
