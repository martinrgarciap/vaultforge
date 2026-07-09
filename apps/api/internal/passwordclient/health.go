package passwordclient

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const ServiceName = "vaultforge.password.v1.PasswordService"

type HealthPinger struct {
	healthClient   healthpb.HealthClient
	serviceName    string
	requestTimeout time.Duration
}

func NewHealthPinger(
	connection grpc.ClientConnInterface,
	config Config,
) (*HealthPinger, error) {
	if connection == nil {
		return nil, errors.New("password service connection is required")
	}

	if config.RequestTimeout() <= 0 {
		return nil, errors.New("password service request timeout must be greater than zero")
	}

	return &HealthPinger{
		healthClient:   healthpb.NewHealthClient(connection),
		serviceName:    ServiceName,
		requestTimeout: config.RequestTimeout(),
	}, nil
}

func (pinger *HealthPinger) Ping(
	ctx context.Context,
) error {
	if pinger == nil || pinger.healthClient == nil {
		return errors.New("password service unavailable")
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		pinger.requestTimeout,
	)
	defer cancel()

	response, err := pinger.healthClient.Check(
		ctx,
		&healthpb.HealthCheckRequest{
			Service: pinger.serviceName,
		},
	)
	if err != nil {
		return errors.New("password service unavailable")
	}

	if response.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return errors.New("password service unavailable")
	}

	return nil
}
