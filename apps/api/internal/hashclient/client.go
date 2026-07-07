package hashclient

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/hashpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Client struct {
	hashService    hashpb.HashServiceClient
	requestTimeout time.Duration
}

func New(
	hashService hashpb.HashServiceClient,
	config Config,
) (*Client, error) {
	if hashService == nil {
		return nil, errors.New("hash service client is required")
	}

	if config.RequestTimeout() <= 0 {
		return nil, errors.New("hash service request timeout must be greater than zero")
	}

	return &Client{
		hashService:    hashService,
		requestTimeout: config.RequestTimeout(),
	}, nil
}

func (client *Client) Hash(
	ctx context.Context,
	password string,
) (auth.PasswordHash, error) {
	ctx, cancel := context.WithTimeout(
		ctx,
		client.requestTimeout,
	)
	defer cancel()

	response, err := client.hashService.HashPassword(
		ctx,
		&hashpb.HashPasswordRequest{
			Password: password,
		},
	)
	if err != nil {
		return auth.PasswordHash{}, mapHashError(err)
	}

	phcHash := strings.TrimSpace(
		response.GetPhcHash(),
	)
	if phcHash == "" {
		return auth.PasswordHash{}, auth.ErrPasswordHasherUnavailable
	}

	return auth.PasswordHash{
		Encoded:   phcHash,
		Algorithm: auth.AlgorithmArgon2id,
	}, nil
}

func (client *Client) Verify(
	ctx context.Context,
	password string,
	passwordHash auth.PasswordHash,
) error {
	if passwordHash.Algorithm != auth.AlgorithmArgon2id {
		return auth.ErrPasswordAlgorithmUnsupported
	}

	phcHash := strings.TrimSpace(
		passwordHash.Encoded,
	)
	if phcHash == "" {
		return auth.ErrPasswordHashMalformed
	}

	ctx, cancel := context.WithTimeout(
		ctx,
		client.requestTimeout,
	)
	defer cancel()

	response, err := client.hashService.VerifyPassword(
		ctx,
		&hashpb.VerifyPasswordRequest{
			Password: password,
			PhcHash:  phcHash,
		},
	)
	if err != nil {
		return mapVerifyError(err)
	}

	if !response.GetVerified() {
		return auth.ErrPasswordMismatch
	}

	return nil
}

func mapHashError(
	err error,
) error {
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}

	return auth.ErrPasswordHasherUnavailable
}

func mapVerifyError(
	err error,
) error {
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}

	if status.Code(err) == codes.InvalidArgument {
		return auth.ErrPasswordHashMalformed
	}

	return auth.ErrPasswordHasherUnavailable
}

func mapContextError(
	err error,
) error {
	switch {
	case errors.Is(err, context.Canceled):
		return context.Canceled

	case errors.Is(err, context.DeadlineExceeded):
		return context.DeadlineExceeded
	}

	switch status.Code(err) {
	case codes.Canceled:
		return context.Canceled

	case codes.DeadlineExceeded:
		return context.DeadlineExceeded

	default:
		return nil
	}
}
