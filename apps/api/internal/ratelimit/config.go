package ratelimit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

const MinIdentityHMACKeyBytes = 32

var (
	ErrUnavailable = errors.New(
		"rate limit service unavailable",
	)

	ErrInvalidConfiguration = errors.New(
		"rate limit configuration invalid",
	)
)

type Policy struct {
	limit  int64
	window time.Duration
}

func NewPolicy(
	limit int64,
	window time.Duration,
) (Policy, error) {
	policy := Policy{
		limit:  limit,
		window: window,
	}

	if err := policy.validate(); err != nil {
		return Policy{}, err
	}

	return policy, nil
}

func (policy Policy) Limit() int64 {
	return policy.limit
}

func (policy Policy) Window() time.Duration {
	return policy.window
}

func (policy Policy) validate() error {
	if policy.limit <= 0 {
		return fmt.Errorf(
			"%w: limit must be greater than zero",
			ErrInvalidConfiguration,
		)
	}

	if policy.window < time.Millisecond {
		return fmt.Errorf(
			"%w: window must be at least one millisecond",
			ErrInvalidConfiguration,
		)
	}

	return nil
}

type LoginPolicy struct {
	failureLimit  int64
	failureWindow time.Duration
	lockout       time.Duration
}

func NewLoginPolicy(
	failureLimit int64,
	failureWindow time.Duration,
	lockout time.Duration,
) (LoginPolicy, error) {
	policy := LoginPolicy{
		failureLimit:  failureLimit,
		failureWindow: failureWindow,
		lockout:       lockout,
	}

	if err := policy.validate(); err != nil {
		return LoginPolicy{}, err
	}

	return policy, nil
}

func (policy LoginPolicy) FailureLimit() int64 {
	return policy.failureLimit
}

func (policy LoginPolicy) FailureWindow() time.Duration {
	return policy.failureWindow
}

func (policy LoginPolicy) LockoutDuration() time.Duration {
	return policy.lockout
}

func (policy LoginPolicy) validate() error {
	if policy.failureLimit <= 0 {
		return fmt.Errorf(
			"%w: failure limit must be greater than zero",
			ErrInvalidConfiguration,
		)
	}

	if policy.failureWindow < time.Millisecond {
		return fmt.Errorf(
			"%w: failure window must be at least one millisecond",
			ErrInvalidConfiguration,
		)
	}

	if policy.lockout < time.Millisecond {
		return fmt.Errorf(
			"%w: lockout must be at least one millisecond",
			ErrInvalidConfiguration,
		)
	}

	return nil
}

type KeyBuilder struct {
	prefix string
	key    []byte
}

func NewKeyBuilder(
	prefix string,
	key []byte,
) (*KeyBuilder, error) {
	if !validKeyComponent(prefix) {
		return nil, fmt.Errorf(
			"%w: key prefix is invalid",
			ErrInvalidConfiguration,
		)
	}

	if len(key) < MinIdentityHMACKeyBytes {
		return nil, fmt.Errorf(
			"%w: identity HMAC key must contain at least %d bytes",
			ErrInvalidConfiguration,
			MinIdentityHMACKeyBytes,
		)
	}

	return &KeyBuilder{
		prefix: prefix,
		key:    append([]byte(nil), key...),
	}, nil
}

func (builder *KeyBuilder) Key(
	scope string,
	identityParts ...string,
) (string, error) {
	if builder == nil ||
		!validKeyComponent(builder.prefix) ||
		len(builder.key) < MinIdentityHMACKeyBytes {
		return "", ErrInvalidConfiguration
	}

	if !validKeyComponent(scope) {
		return "", fmt.Errorf(
			"%w: scope is invalid",
			ErrInvalidConfiguration,
		)
	}

	if len(identityParts) == 0 {
		return "", fmt.Errorf(
			"%w: identity is required",
			ErrInvalidConfiguration,
		)
	}

	mac := hmac.New(
		sha256.New,
		builder.key,
	)

	_, _ = mac.Write([]byte(scope))
	_, _ = mac.Write([]byte{0})

	for _, part := range identityParts {
		if part == "" {
			return "", fmt.Errorf(
				"%w: identity parts must not be empty",
				ErrInvalidConfiguration,
			)
		}

		var length [8]byte

		binary.BigEndian.PutUint64(
			length[:],
			uint64(len(part)),
		)

		_, _ = mac.Write(length[:])
		_, _ = mac.Write([]byte(part))
	}

	digest := base64.RawURLEncoding.EncodeToString(
		mac.Sum(nil),
	)

	return builder.prefix +
		":" +
		scope +
		":" +
		digest, nil
}

func validKeyComponent(value string) bool {
	if len(value) == 0 || len(value) > 64 {
		return false
	}

	for _, character := range value {
		switch {
		case character >= 'a' &&
			character <= 'z':
		case character >= '0' &&
			character <= '9':
		case character == ':',
			character == '-',
			character == '_':
		default:
			return false
		}
	}

	return true
}
