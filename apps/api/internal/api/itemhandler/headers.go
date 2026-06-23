package itemhandler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const (
	maxIfMatchHeaderBytes           = 32
	softDeleteExpectedVersionHeader = "X-VaultForge-Expected-Version"
)

var (
	errIfMatchRequired        = errors.New("If-Match header is required")
	errIfMatchInvalid         = errors.New("If-Match header is invalid")
	errIdempotencyKeyRequired = errors.New("Idempotency-Key header is required")
	errIdempotencyKeyInvalid  = errors.New("Idempotency-Key header is invalid")
)

func itemETag(version int) string {
	if version < vault.InitialItemVersion {
		return ""
	}

	return `"` + strconv.Itoa(version) + `"`
}

func expectedItemVersion(r *http.Request) (int, error) {
	return expectedItemVersionFromHeader(r, "If-Match")
}

func expectedSoftDeleteItemVersion(r *http.Request) (int, error) {
	if r == nil {
		return 0, errIfMatchRequired
	}

	if len(r.Header.Values("If-Match")) > 0 {
		return expectedItemVersion(r)
	}

	return expectedItemVersionFromHeader(
		r,
		softDeleteExpectedVersionHeader,
	)
}

func expectedItemVersionFromHeader(
	r *http.Request,
	headerName string,
) (int, error) {
	if r == nil {
		return 0, errIfMatchRequired
	}

	values := r.Header.Values(headerName)

	if len(values) == 0 {
		return 0, errIfMatchRequired
	}

	if len(values) != 1 {
		return 0, errIfMatchInvalid
	}

	return parseItemETag(values[0])
}

func parseItemETag(value string) (int, error) {
	if len(value) > maxIfMatchHeaderBytes {
		return 0, errIfMatchInvalid
	}

	normalized := strings.TrimSpace(value)

	if len(normalized) < 3 ||
		normalized[0] != '"' ||
		normalized[len(normalized)-1] != '"' {
		return 0, errIfMatchInvalid
	}

	versionText := normalized[1 : len(normalized)-1]

	if versionText == "" ||
		strings.TrimSpace(versionText) != versionText {
		return 0, errIfMatchInvalid
	}

	version, err := strconv.Atoi(versionText)
	if err != nil ||
		version < vault.InitialItemVersion ||
		strconv.Itoa(version) != versionText {
		return 0, errIfMatchInvalid
	}

	return version, nil
}

func idempotencyKey(r *http.Request) (string, error) {
	if r == nil {
		return "", errIdempotencyKeyRequired
	}

	values := r.Header.Values("Idempotency-Key")

	if len(values) == 0 {
		return "", errIdempotencyKeyRequired
	}

	if len(values) != 1 {
		return "", errIdempotencyKeyInvalid
	}

	normalized := strings.TrimSpace(values[0])

	if normalized == "" ||
		len(normalized) > vault.MaxItemIdempotencyKeyBytes {
		return "", errIdempotencyKeyInvalid
	}

	return normalized, nil
}
