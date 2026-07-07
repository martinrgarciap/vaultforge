package hashclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/auth"
	"github.com/martinrgarciap/vaultforge/apps/api/internal/hashpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const testPassword = "correct horse battery staple"

func TestClientHash(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		hashResponse: &hashpb.HashPasswordResponse{
			PhcHash: "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		},
	}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	passwordHash, err := client.Hash(
		context.Background(),
		testPassword,
	)
	if err != nil {
		t.Fatalf("Hash() returned unexpected error: %v", err)
	}

	if passwordHash.Algorithm != auth.AlgorithmArgon2id {
		t.Fatalf(
			"Hash() algorithm = %q, want %q",
			passwordHash.Algorithm,
			auth.AlgorithmArgon2id,
		)
	}

	if passwordHash.Encoded != fake.hashResponse.GetPhcHash() {
		t.Fatalf(
			"Hash() encoded = %q, want %q",
			passwordHash.Encoded,
			fake.hashResponse.GetPhcHash(),
		)
	}

	if fake.hashPassword != testPassword {
		t.Fatalf(
			"HashPassword request password = %q, want %q",
			fake.hashPassword,
			testPassword,
		)
	}
}

func TestClientHashMapsUnavailableService(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		hashErr: status.Error(
			codes.Unavailable,
			"service unavailable",
		),
	}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	_, err := client.Hash(
		context.Background(),
		testPassword,
	)

	if !errors.Is(err, auth.ErrPasswordHasherUnavailable) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			auth.ErrPasswordHasherUnavailable,
		)
	}
}

func TestClientHashRejectsEmptyRemoteHash(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		hashResponse: &hashpb.HashPasswordResponse{
			PhcHash: " ",
		},
	}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	_, err := client.Hash(
		context.Background(),
		testPassword,
	)

	if !errors.Is(err, auth.ErrPasswordHasherUnavailable) {
		t.Fatalf(
			"Hash() error = %v, want %v",
			err,
			auth.ErrPasswordHasherUnavailable,
		)
	}
}

func TestClientVerify(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		verifyResponse: &hashpb.VerifyPasswordResponse{
			Verified: true,
		},
	}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	passwordHash := auth.PasswordHash{
		Encoded:   "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
		Algorithm: auth.AlgorithmArgon2id,
	}

	err := client.Verify(
		context.Background(),
		testPassword,
		passwordHash,
	)
	if err != nil {
		t.Fatalf("Verify() returned unexpected error: %v", err)
	}

	if fake.verifyPassword != testPassword {
		t.Fatalf(
			"VerifyPassword request password = %q, want %q",
			fake.verifyPassword,
			testPassword,
		)
	}

	if fake.verifyPhcHash != passwordHash.Encoded {
		t.Fatalf(
			"VerifyPassword request hash = %q, want %q",
			fake.verifyPhcHash,
			passwordHash.Encoded,
		)
	}
}

func TestClientVerifyMapsWrongPassword(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		verifyResponse: &hashpb.VerifyPasswordResponse{
			Verified: false,
		},
	}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	err := client.Verify(
		context.Background(),
		testPassword,
		auth.PasswordHash{
			Encoded:   "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
			Algorithm: auth.AlgorithmArgon2id,
		},
	)

	if !errors.Is(err, auth.ErrPasswordMismatch) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			auth.ErrPasswordMismatch,
		)
	}
}

func TestClientVerifyRejectsUnsupportedAlgorithm(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	err := client.Verify(
		context.Background(),
		testPassword,
		auth.PasswordHash{
			Encoded:   "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
			Algorithm: "bcrypt",
		},
	)

	if !errors.Is(err, auth.ErrPasswordAlgorithmUnsupported) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			auth.ErrPasswordAlgorithmUnsupported,
		)
	}

	if fake.verifyCalls != 0 {
		t.Fatalf(
			"VerifyPassword calls = %d, want 0",
			fake.verifyCalls,
		)
	}
}

func TestClientVerifyRejectsEmptyStoredHash(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	err := client.Verify(
		context.Background(),
		testPassword,
		auth.PasswordHash{
			Encoded:   " ",
			Algorithm: auth.AlgorithmArgon2id,
		},
	)

	if !errors.Is(err, auth.ErrPasswordHashMalformed) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			auth.ErrPasswordHashMalformed,
		)
	}

	if fake.verifyCalls != 0 {
		t.Fatalf(
			"VerifyPassword calls = %d, want 0",
			fake.verifyCalls,
		)
	}
}

func TestClientVerifyMapsMalformedHash(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		verifyErr: status.Error(
			codes.InvalidArgument,
			"stored hash is malformed",
		),
	}

	client := newTestClient(
		t,
		fake,
		5*time.Second,
	)

	err := client.Verify(
		context.Background(),
		testPassword,
		auth.PasswordHash{
			Encoded:   "not-a-phc-hash",
			Algorithm: auth.AlgorithmArgon2id,
		},
	)

	if !errors.Is(err, auth.ErrPasswordHashMalformed) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			auth.ErrPasswordHashMalformed,
		)
	}
}

func TestClientVerifyMapsDeadlineExceeded(t *testing.T) {
	t.Parallel()

	fake := &fakeHashServiceClient{
		delay: 100 * time.Millisecond,
	}

	client := newTestClient(
		t,
		fake,
		10*time.Millisecond,
	)

	err := client.Verify(
		context.Background(),
		testPassword,
		auth.PasswordHash{
			Encoded:   "$argon2id$v=19$m=19456,t=2,p=1$c2FsdA$aGFzaA",
			Algorithm: auth.AlgorithmArgon2id,
		},
	)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf(
			"Verify() error = %v, want %v",
			err,
			context.DeadlineExceeded,
		)
	}
}

func newTestClient(
	t *testing.T,
	fake *fakeHashServiceClient,
	requestTimeout time.Duration,
) *Client {
	t.Helper()

	config, err := NewConfig(
		DefaultAddress,
		DefaultDialTimeout,
		requestTimeout,
	)
	if err != nil {
		t.Fatalf("NewConfig() returned unexpected error: %v", err)
	}

	client, err := New(
		fake,
		config,
	)
	if err != nil {
		t.Fatalf("New() returned unexpected error: %v", err)
	}

	return client
}

type fakeHashServiceClient struct {
	hashResponse   *hashpb.HashPasswordResponse
	hashErr        error
	hashPassword   string
	hashCalls      int
	verifyResponse *hashpb.VerifyPasswordResponse
	verifyErr      error
	verifyPassword string
	verifyPhcHash  string
	verifyCalls    int
	delay          time.Duration
}

func (fake *fakeHashServiceClient) HashPassword(
	ctx context.Context,
	request *hashpb.HashPasswordRequest,
	_ ...grpc.CallOption,
) (*hashpb.HashPasswordResponse, error) {
	fake.hashCalls++
	fake.hashPassword = request.GetPassword()

	if err := fake.wait(ctx); err != nil {
		return nil, err
	}

	if fake.hashErr != nil {
		return nil, fake.hashErr
	}

	if fake.hashResponse != nil {
		return fake.hashResponse, nil
	}

	return &hashpb.HashPasswordResponse{}, nil
}

func (fake *fakeHashServiceClient) VerifyPassword(
	ctx context.Context,
	request *hashpb.VerifyPasswordRequest,
	_ ...grpc.CallOption,
) (*hashpb.VerifyPasswordResponse, error) {
	fake.verifyCalls++
	fake.verifyPassword = request.GetPassword()
	fake.verifyPhcHash = request.GetPhcHash()

	if err := fake.wait(ctx); err != nil {
		return nil, err
	}

	if fake.verifyErr != nil {
		return nil, fake.verifyErr
	}

	if fake.verifyResponse != nil {
		return fake.verifyResponse, nil
	}

	return &hashpb.VerifyPasswordResponse{}, nil
}

func (fake *fakeHashServiceClient) wait(
	ctx context.Context,
) error {
	if fake.delay <= 0 {
		return nil
	}

	timer := time.NewTimer(
		fake.delay,
	)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-timer.C:
		return nil
	}
}
