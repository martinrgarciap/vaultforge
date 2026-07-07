package hashclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func TestHealthPingerPingPassesWhenServiceIsServing(t *testing.T) {
	t.Parallel()

	fake := &fakeHealthClient{
		response: &healthpb.HealthCheckResponse{
			Status: healthpb.HealthCheckResponse_SERVING,
		},
	}

	pinger := &HealthPinger{
		healthClient:   fake,
		serviceName:    ServiceName,
		requestTimeout: time.Second,
	}

	if err := pinger.Ping(context.Background()); err != nil {
		t.Fatalf("Ping() returned unexpected error: %v", err)
	}

	if fake.service != ServiceName {
		t.Fatalf("health check service = %q, want %q", fake.service, ServiceName)
	}
}

func TestHealthPingerPingFailsWhenServiceIsNotServing(t *testing.T) {
	t.Parallel()

	fake := &fakeHealthClient{
		response: &healthpb.HealthCheckResponse{
			Status: healthpb.HealthCheckResponse_NOT_SERVING,
		},
	}

	pinger := &HealthPinger{
		healthClient:   fake,
		serviceName:    ServiceName,
		requestTimeout: time.Second,
	}

	err := pinger.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() returned nil error, want dependency error")
	}

	if err.Error() != "hash service unavailable" {
		t.Fatalf("Ping() error = %q, want safe dependency error", err.Error())
	}
}

func TestHealthPingerPingFailsSafelyWhenHealthCheckFails(t *testing.T) {
	t.Parallel()

	fake := &fakeHealthClient{
		err: errors.New("synthetic internal hash service details"),
	}

	pinger := &HealthPinger{
		healthClient:   fake,
		serviceName:    ServiceName,
		requestTimeout: time.Second,
	}

	err := pinger.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() returned nil error, want dependency error")
	}

	if err.Error() != "hash service unavailable" {
		t.Fatalf("Ping() error = %q, want safe dependency error", err.Error())
	}
}

func TestHealthPingerPingRejectsMissingClient(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		pinger *HealthPinger
	}{
		{
			name:   "nil pinger",
			pinger: nil,
		},
		{
			name: "nil health client",
			pinger: &HealthPinger{
				requestTimeout: time.Second,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			err := testCase.pinger.Ping(context.Background())
			if err == nil {
				t.Fatal("Ping() returned nil error, want dependency error")
			}

			if err.Error() != "hash service unavailable" {
				t.Fatalf("Ping() error = %q, want safe dependency error", err.Error())
			}
		})
	}
}

func TestNewHealthPingerRejectsInvalidConfig(t *testing.T) {
	t.Parallel()

	config := Config{}

	_, err := NewHealthPinger(
		&fakeClientConn{},
		config,
	)
	if err == nil {
		t.Fatal("NewHealthPinger() returned nil error, want validation error")
	}
}

type fakeHealthClient struct {
	response *healthpb.HealthCheckResponse
	err      error
	service  string
}

func (fake *fakeHealthClient) Check(
	_ context.Context,
	request *healthpb.HealthCheckRequest,
	_ ...grpc.CallOption,
) (*healthpb.HealthCheckResponse, error) {
	fake.service = request.GetService()

	if fake.err != nil {
		return nil, fake.err
	}

	if fake.response != nil {
		return fake.response, nil
	}

	return &healthpb.HealthCheckResponse{}, nil
}

func (fake *fakeHealthClient) Watch(
	context.Context,
	*healthpb.HealthCheckRequest,
	...grpc.CallOption,
) (healthpb.Health_WatchClient, error) {
	return nil, errors.New("watch is not used")
}

func (fake *fakeHealthClient) List(
	context.Context,
	*healthpb.HealthListRequest,
	...grpc.CallOption,
) (*healthpb.HealthListResponse, error) {
	return nil, errors.New("list is not used")
}

type fakeClientConn struct{}

func (fake *fakeClientConn) Invoke(
	context.Context,
	string,
	any,
	any,
	...grpc.CallOption,
) error {
	return nil
}

func (fake *fakeClientConn) NewStream(
	context.Context,
	*grpc.StreamDesc,
	string,
	...grpc.CallOption,
) (grpc.ClientStream, error) {
	return nil, errors.New("stream is not used")
}
