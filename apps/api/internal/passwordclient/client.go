package passwordclient

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/passwordpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrPasswordServiceUnavailable = errors.New("password service unavailable")
	ErrPasswordRequestInvalid     = errors.New("password request invalid")
	ErrPasswordResponseInvalid    = errors.New("password service returned invalid response")
)

type GenerateInput struct {
	Length           uint32
	IncludeUppercase bool
	IncludeLowercase bool
	IncludeDigits    bool
	IncludeSymbols   bool
	ExcludeChars     string
}

type GenerateResult struct {
	Password    string
	EntropyBits float64
}

type CheckStrengthInput struct {
	Password string
}

type CheckStrengthResult struct {
	Score             uint32
	Label             string
	EntropyBits       float64
	CrackTimeEstimate string
	Suggestions       []string
}

type Client struct {
	passwordService passwordpb.PasswordServiceClient
	requestTimeout  time.Duration
}

func New(
	passwordService passwordpb.PasswordServiceClient,
	config Config,
) (*Client, error) {
	if passwordService == nil {
		return nil, errors.New("password service client is required")
	}

	if config.RequestTimeout() <= 0 {
		return nil, errors.New("password service request timeout must be greater than zero")
	}

	return &Client{
		passwordService: passwordService,
		requestTimeout:  config.RequestTimeout(),
	}, nil
}

func (client *Client) Generate(
	ctx context.Context,
	input GenerateInput,
) (GenerateResult, error) {
	ctx, cancel := context.WithTimeout(
		ctx,
		client.requestTimeout,
	)
	defer cancel()

	response, err := client.passwordService.Generate(
		ctx,
		&passwordpb.GenerateRequest{
			Length:           input.Length,
			IncludeUppercase: input.IncludeUppercase,
			IncludeLowercase: input.IncludeLowercase,
			IncludeDigits:    input.IncludeDigits,
			IncludeSymbols:   input.IncludeSymbols,
			ExcludeChars:     input.ExcludeChars,
		},
	)
	if err != nil {
		return GenerateResult{}, mapPasswordServiceError(err)
	}

	if response.GetPassword() == "" || response.GetEntropyBits() < 0 {
		return GenerateResult{}, ErrPasswordResponseInvalid
	}

	return GenerateResult{
		Password:    response.GetPassword(),
		EntropyBits: response.GetEntropyBits(),
	}, nil
}

func (client *Client) CheckStrength(
	ctx context.Context,
	input CheckStrengthInput,
) (CheckStrengthResult, error) {
	ctx, cancel := context.WithTimeout(
		ctx,
		client.requestTimeout,
	)
	defer cancel()

	response, err := client.passwordService.CheckStrength(
		ctx,
		&passwordpb.CheckStrengthRequest{
			Password: input.Password,
		},
	)
	if err != nil {
		return CheckStrengthResult{}, mapPasswordServiceError(err)
	}

	if response.GetScore() > 4 ||
		strings.TrimSpace(response.GetLabel()) == "" ||
		response.GetEntropyBits() < 0 ||
		strings.TrimSpace(response.GetCrackTimeEstimate()) == "" {
		return CheckStrengthResult{}, ErrPasswordResponseInvalid
	}

	return CheckStrengthResult{
		Score:             response.GetScore(),
		Label:             strings.TrimSpace(response.GetLabel()),
		EntropyBits:       response.GetEntropyBits(),
		CrackTimeEstimate: strings.TrimSpace(response.GetCrackTimeEstimate()),
		Suggestions:       append([]string(nil), response.GetSuggestions()...),
	}, nil
}

func mapPasswordServiceError(
	err error,
) error {
	if mapped := mapContextError(err); mapped != nil {
		return mapped
	}

	if status.Code(err) == codes.InvalidArgument {
		return ErrPasswordRequestInvalid
	}

	return ErrPasswordServiceUnavailable
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
