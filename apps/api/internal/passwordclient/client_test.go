package passwordclient

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/passwordpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakePasswordServiceClient struct {
	generateResponse *passwordpb.GenerateResponse
	strengthResponse *passwordpb.CheckStrengthResponse
	err              error

	generateCalls       int
	lastGenerateRequest *passwordpb.GenerateRequest

	strengthCalls       int
	lastStrengthRequest *passwordpb.CheckStrengthRequest
}

func (client *fakePasswordServiceClient) Generate(
	_ context.Context,
	request *passwordpb.GenerateRequest,
	_ ...grpc.CallOption,
) (*passwordpb.GenerateResponse, error) {
	client.generateCalls++
	client.lastGenerateRequest = request

	if client.err != nil {
		return nil, client.err
	}

	return client.generateResponse, nil
}

func (client *fakePasswordServiceClient) CheckStrength(
	_ context.Context,
	request *passwordpb.CheckStrengthRequest,
	_ ...grpc.CallOption,
) (*passwordpb.CheckStrengthResponse, error) {
	client.strengthCalls++
	client.lastStrengthRequest = request

	if client.err != nil {
		return nil, client.err
	}

	return client.strengthResponse, nil
}

func TestGenerateCallsPasswordService(t *testing.T) {
	t.Parallel()

	fake := &fakePasswordServiceClient{
		generateResponse: &passwordpb.GenerateResponse{
			Password:    "A1!synthetic-demo",
			EntropyBits: 96.5,
		},
	}

	client := &Client{
		passwordService: fake,
		requestTimeout:  time.Second,
	}

	result, err := client.Generate(
		context.Background(),
		GenerateInput{
			Length:           18,
			IncludeUppercase: true,
			IncludeLowercase: true,
			IncludeDigits:    true,
			IncludeSymbols:   true,
			ExcludeChars:     "O0l1",
		},
	)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if result.Password != "A1!synthetic-demo" {
		t.Fatalf("generated password = %q", result.Password)
	}

	if result.EntropyBits != 96.5 {
		t.Fatalf("entropy bits = %v, want 96.5", result.EntropyBits)
	}

	if fake.generateCalls != 1 {
		t.Fatalf("generate calls = %d, want 1", fake.generateCalls)
	}

	if fake.lastGenerateRequest.GetLength() != 18 {
		t.Fatalf("request length = %d, want 18", fake.lastGenerateRequest.GetLength())
	}

	if !fake.lastGenerateRequest.GetIncludeSymbols() {
		t.Fatal("request include symbols = false, want true")
	}

	if fake.lastGenerateRequest.GetExcludeChars() != "O0l1" {
		t.Fatalf("request exclude chars = %q", fake.lastGenerateRequest.GetExcludeChars())
	}
}

func TestCheckStrengthCallsPasswordService(t *testing.T) {
	t.Parallel()

	fake := &fakePasswordServiceClient{
		strengthResponse: &passwordpb.CheckStrengthResponse{
			Score:             4,
			Label:             "very strong",
			EntropyBits:       110.25,
			CrackTimeEstimate: "centuries",
			Suggestions: []string{
				"Use a password manager.",
			},
		},
	}

	client := &Client{
		passwordService: fake,
		requestTimeout:  time.Second,
	}

	result, err := client.CheckStrength(
		context.Background(),
		CheckStrengthInput{
			Password: "synthetic-only-password",
		},
	)
	if err != nil {
		t.Fatalf("CheckStrength() error = %v", err)
	}

	if result.Score != 4 {
		t.Fatalf("score = %d, want 4", result.Score)
	}

	if result.Label != "very strong" {
		t.Fatalf("label = %q, want very strong", result.Label)
	}

	if result.EntropyBits != 110.25 {
		t.Fatalf("entropy bits = %v, want 110.25", result.EntropyBits)
	}

	if result.CrackTimeEstimate != "centuries" {
		t.Fatalf("crack time estimate = %q, want centuries", result.CrackTimeEstimate)
	}

	if len(result.Suggestions) != 1 ||
		result.Suggestions[0] != "Use a password manager." {
		t.Fatalf("suggestions = %#v", result.Suggestions)
	}

	if fake.strengthCalls != 1 {
		t.Fatalf("strength calls = %d, want 1", fake.strengthCalls)
	}

	if fake.lastStrengthRequest.GetPassword() != "synthetic-only-password" {
		t.Fatal("strength request password was not forwarded")
	}
}

func TestPasswordServiceInvalidArgumentMapsToInvalidRequest(t *testing.T) {
	t.Parallel()

	fake := &fakePasswordServiceClient{
		err: status.Error(codes.InvalidArgument, "invalid request"),
	}

	client := &Client{
		passwordService: fake,
		requestTimeout:  time.Second,
	}

	_, err := client.Generate(context.Background(), GenerateInput{})
	if !errors.Is(err, ErrPasswordRequestInvalid) {
		t.Fatalf("Generate() error = %v, want ErrPasswordRequestInvalid", err)
	}
}

func TestPasswordServiceUnavailableMapsToUnavailable(t *testing.T) {
	t.Parallel()

	fake := &fakePasswordServiceClient{
		err: status.Error(codes.Unavailable, "service unavailable"),
	}

	client := &Client{
		passwordService: fake,
		requestTimeout:  time.Second,
	}

	_, err := client.CheckStrength(
		context.Background(),
		CheckStrengthInput{
			Password: "synthetic-only-password",
		},
	)
	if !errors.Is(err, ErrPasswordServiceUnavailable) {
		t.Fatalf("CheckStrength() error = %v, want ErrPasswordServiceUnavailable", err)
	}
}

func TestNewRejectsNilServiceClient(t *testing.T) {
	t.Parallel()

	_, err := New(nil, Config{requestTimeout: time.Second})
	if err == nil {
		t.Fatal("New() error = nil, want error")
	}
}

func TestNewRejectsInvalidTimeout(t *testing.T) {
	t.Parallel()

	_, err := New(
		&fakePasswordServiceClient{},
		Config{},
	)
	if err == nil {
		t.Fatal("New() error = nil, want error")
	}
}
