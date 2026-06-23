package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"
)

func TestRegistryUsesRoutePatternWithoutResourceIDs(t *testing.T) {
	t.Parallel()

	registry := New(buildinfo.New("v1.2.3", "abcdef123456"))

	router := chi.NewRouter()
	router.Use(registry.Middleware)

	router.Get("/v1/vaults/{vaultID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	const (
		vaultID     = "sensitive-vault-identifier"
		secretQuery = "synthetic-secret-query"
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/"+vaultID+"?value="+secretQuery,
		nil,
	)
	request.Header.Set("Authorization", "Bearer synthetic-secret-token")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	metricsRecorder := httptest.NewRecorder()
	registry.ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))

	output := metricsRecorder.Body.String()
	expected := `vaultforge_http_requests_total{method="GET",route="/v1/vaults/{vaultID}",status_class="2xx"} 1`

	if !strings.Contains(output, expected) {
		t.Fatalf("metrics output omitted normalized route:\n%s", output)
	}

	for _, forbidden := range []string{
		vaultID,
		secretQuery,
		"synthetic-secret-token",
		"Authorization",
	} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("metrics exposed %q", forbidden)
		}
	}

	if metricsRecorder.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("metrics response did not disable caching")
	}
}

func TestRegistryRecordsRequestsConcurrently(t *testing.T) {
	t.Parallel()

	registry := New(buildinfo.New("development", "abcdef123456"))

	router := chi.NewRouter()
	router.Use(registry.Middleware)
	router.Get("/health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const requestCount = 100

	var wait sync.WaitGroup

	for index := 0; index < requestCount; index++ {
		wait.Add(1)

		go func() {
			defer wait.Done()

			request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)
		}()
	}

	wait.Wait()

	metricsRecorder := httptest.NewRecorder()
	registry.ServeHTTP(metricsRecorder, httptest.NewRequest(http.MethodGet, "/internal/metrics", nil))

	expected := `vaultforge_http_requests_total{method="GET",route="/health/live",status_class="2xx"} 100`

	if !strings.Contains(metricsRecorder.Body.String(), expected) {
		t.Fatalf("concurrent count was incorrect:\n%s", metricsRecorder.Body.String())
	}
}

func TestRegistryNormalizesUnknownMethods(t *testing.T) {
	t.Parallel()

	if normalizeMethod("CUSTOM") != "OTHER" {
		t.Fatal("unknown HTTP method was not normalized")
	}
}
