package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTraceRequestsUsesNormalizedRouteWithoutSensitiveData(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	t.Cleanup(func() {
		_ = provider.Shutdown(t.Context())
	})

	router := chi.NewRouter()
	router.Use(traceRequests("vaultforge-api", provider, propagation.TraceContext{}))

	router.Get("/v1/vaults/{vaultID}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	const vaultID = "08dd943e-2066-4d6c-b5c2-780058990b9f"
	const token = "synthetic-super-secret-access-token"

	request := httptest.NewRequest(http.MethodGet, "/v1/vaults/"+vaultID+"?include=metadata", nil)
	request.Header.Set("Authorization", "Bearer "+token)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	spans := exporter.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("span count = %d, want 1", len(spans))
	}

	span := spans[0]
	if span.Name != "GET /v1/vaults/{vaultID}" {
		t.Fatalf("span name = %q", span.Name)
	}

	spanOutput := span.Name + fmt.Sprint(span.Attributes)

	for _, forbidden := range []string{
		vaultID,
		"include=metadata",
		token,
		"Authorization",
	} {
		if strings.Contains(spanOutput, forbidden) {
			t.Fatalf("span exposed %q", forbidden)
		}
	}
}
