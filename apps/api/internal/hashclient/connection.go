package hashclient

import (
	"context"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

func Dial(
	ctx context.Context,
	config Config,
) (*grpc.ClientConn, error) {
	if config.Address() == "" {
		return nil, errors.New("hash service address is required")
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		config.DialTimeout(),
	)
	defer cancel()

	connection, err := grpc.NewClient(
		config.Address(),
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		return nil, errors.New("hash service client initialization failed")
	}

	connection.Connect()

	for {
		state := connection.GetState()
		if state == connectivity.Ready {
			return connection, nil
		}

		if !connection.WaitForStateChange(ctx, state) {
			_ = connection.Close()

			return nil, errors.New("hash service connection unavailable")
		}
	}
}
