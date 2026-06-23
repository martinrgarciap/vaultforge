package diagnostics

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
)

func TestHandlerReturnsSafeBuildMetadata(t *testing.T) {
	t.Parallel()

	handler := New(buildinfo.New("v1.2.3", "abcdef123456"))

	request := httptest.NewRequest(http.MethodGet, "/health/diagnostics", nil)
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if recorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("diagnostics response did not disable caching")
	}

	var body buildinfo.Info

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decode diagnostics response: %v", err)
	}

	if body.Service != buildinfo.ServiceName ||
		body.Version != "v1.2.3" ||
		body.Commit != "abcdef123456" {
		t.Fatalf("diagnostics response = %+v", body)
	}
}

func TestHandlerDoesNotExposeUnsafeBuildInput(t *testing.T) {
	t.Parallel()

	const secretMarker = "synthetic-build-secret-marker"

	handler := New(buildinfo.Info{
		Version: "version " + secretMarker,
		Commit:  "commit " + secretMarker,
	})

	request := httptest.NewRequest(http.MethodGet, "/health/diagnostics", nil)
	recorder := httptest.NewRecorder()

	handler.Get(recorder, request)

	if strings.Contains(recorder.Body.String(), secretMarker) {
		t.Fatal("diagnostics response exposed unsafe metadata")
	}
}
