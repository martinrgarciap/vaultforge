package telemetry

import (
	"context"
	"errors"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/buildinfo"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

type Shutdown func(context.Context) error

func Start(
	ctx context.Context,
	config Config,
	build buildinfo.Info,
	environment string,
	logger *zap.SugaredLogger,
) (Shutdown, error) {
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	otel.SetErrorHandler(telemetryErrorHandler{logger: logger})

	if !config.Enabled() {
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(config.Endpoint()))
	if err != nil {
		return nil, errors.New("unable to initialize OpenTelemetry exporter")
	}

	serviceResource := resource.NewWithAttributes(
		"",
		attribute.String("service.name", build.Service),
		attribute.String("service.version", build.Version),
		attribute.String("deployment.environment.name", environment),
	)

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(serviceResource),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	otel.SetTracerProvider(provider)

	return provider.Shutdown, nil
}

type telemetryErrorHandler struct {
	logger *zap.SugaredLogger
}

func (handler telemetryErrorHandler) Handle(error) {
	if handler.logger == nil {
		return
	}

	handler.logger.Warnw("OpenTelemetry export failed")
}
