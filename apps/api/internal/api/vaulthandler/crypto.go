package vaulthandler

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/request"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/response"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

type initializeVaultCryptoRequest struct {
	CryptoVersion int16  `json:"cryptoVersion"`
	KDFVersion    int16  `json:"kdfVersion"`
	Salt          string `json:"salt"`
	WrappedKey    string `json:"wrappedKey"`
}

type vaultCryptoMetadataInitializer interface {
	InitializeCryptoMetadata(
		ctx context.Context,
		input vault.InitializeCryptoMetadataInput,
	) (vault.Vault, error)
}

func (handler *Handler) InitializeCrypto(
	w http.ResponseWriter,
	r *http.Request,
) {
	principal, ok := handler.principal(w, r)
	if !ok {
		return
	}

	initializer, ok := handler.vaultService.(vaultCryptoMetadataInitializer)
	if !ok {
		handler.writeError(
			w,
			r,
			http.StatusServiceUnavailable,
			"vault_unavailable",
			"Vault operations are temporarily unavailable.",
		)

		return
	}

	var requestBody initializeVaultCryptoRequest

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

	salt, err := decodeVaultCryptoBytes(requestBody.Salt)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	wrappedKey, err := decodeVaultCryptoBytes(requestBody.WrappedKey)
	if err != nil {
		handler.writeServiceError(w, r, err)
		return
	}

	initializedVault, err := initializer.InitializeCryptoMetadata(
		r.Context(),
		vault.InitializeCryptoMetadataInput{
			OwnerID: principal.UserID,
			VaultID: chi.URLParam(r, "vaultID"),
			Metadata: vault.VaultCryptoMetadata{
				CryptoVersion: requestBody.CryptoVersion,
				KDFVersion:    requestBody.KDFVersion,
				Salt:          salt,
				WrappedKey:    wrappedKey,
			},
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
				initializedVault,
			),
		},
	)
	if err != nil {
		handler.logResponseFailure(r)
	}
}

func decodeVaultCryptoBytes(value string) ([]byte, error) {
	if value == "" || strings.TrimSpace(value) != value {
		return nil, vault.ErrVaultCryptoMetadataInvalid
	}

	decoded, err := base64.StdEncoding.Strict().DecodeString(value)
	if err != nil {
		return nil, vault.ErrVaultCryptoMetadataInvalid
	}

	return decoded, nil
}
