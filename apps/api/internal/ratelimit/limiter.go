package ratelimit

import (
	"context"
	"errors"
	"strconv"
	"time"
)

const (
	ScopeRegistration  = "register"
	ScopeLogin         = "login"
	ScopeRefresh       = "refresh"
	ScopeMutation      = "mutation"
	ScopePasswordTools = "password-tools"

	scopeLoginFailure = "login-failure"
	scopeLoginLockout = "login-lockout"
)

const fixedWindowScript = `
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local window_ms = tonumber(ARGV[2])

local count = redis.call("INCR", key)
local ttl = redis.call("PTTL", key)

if count == 1 or ttl < 0 then
	redis.call("PEXPIRE", key, window_ms)
	ttl = window_ms
end

local allowed = 0

if count <= limit then
	allowed = 1
end

return {allowed, count, ttl}
`

const checkLockoutScript = `
local key = KEYS[1]
local lockout_ms = tonumber(ARGV[1])

local ttl = redis.call("PTTL", key)

if ttl == -2 then
	return {0, 0}
end

if ttl == -1 then
	redis.call("PEXPIRE", key, lockout_ms)
	ttl = lockout_ms
end

if ttl <= 0 then
	return {0, 0}
end

return {1, ttl}
`

const recordLoginFailureScript = `
local failure_key = KEYS[1]
local lockout_key = KEYS[2]

local failure_limit = tonumber(ARGV[1])
local failure_window_ms = tonumber(ARGV[2])
local lockout_ms = tonumber(ARGV[3])

local lockout_ttl = redis.call("PTTL", lockout_key)

if lockout_ttl ~= -2 then
	if lockout_ttl < 0 then
		redis.call("PEXPIRE", lockout_key, lockout_ms)
		lockout_ttl = lockout_ms
	end

	return {1, lockout_ttl, failure_limit}
end

local failures = redis.call("INCR", failure_key)
local failure_ttl = redis.call("PTTL", failure_key)

if failures == 1 or failure_ttl < 0 then
	redis.call("PEXPIRE", failure_key, failure_window_ms)
	failure_ttl = failure_window_ms
end

if failures >= failure_limit then
	redis.call(
		"SET",
		lockout_key,
		"1",
		"PX",
		lockout_ms
	)

	redis.call("DEL", failure_key)

	return {1, lockout_ms, failures}
end

return {0, failure_ttl, failures}
`

const clearLoginProtectionScript = `
return redis.call(
	"DEL",
	KEYS[1],
	KEYS[2]
)
`

type ScriptRunner interface {
	RunScript(
		ctx context.Context,
		source string,
		keys []string,
		args ...any,
	) (any, error)
}

type Decision struct {
	Allowed    bool
	Remaining  int64
	RetryAfter time.Duration
}

type Limiter struct {
	runner  ScriptRunner
	builder *KeyBuilder
}

func NewLimiter(
	runner ScriptRunner,
	builder *KeyBuilder,
) (*Limiter, error) {
	if runner == nil || builder == nil {
		return nil, ErrInvalidConfiguration
	}

	return &Limiter{
		runner:  runner,
		builder: builder,
	}, nil
}

func (limiter *Limiter) Allow(
	ctx context.Context,
	scope string,
	policy Policy,
	identityParts ...string,
) (Decision, error) {
	if err := policy.validate(); err != nil {
		return Decision{}, err
	}

	if ctx == nil ||
		limiter == nil ||
		limiter.runner == nil ||
		limiter.builder == nil {
		return Decision{}, ErrUnavailable
	}

	key, err := limiter.builder.Key(
		scope,
		identityParts...,
	)
	if err != nil {
		return Decision{}, err
	}

	result, err := limiter.runner.RunScript(
		ctx,
		fixedWindowScript,
		[]string{key},
		policy.limit,
		policy.window.Milliseconds(),
	)
	if err != nil {
		return Decision{}, mapRunnerError(err)
	}

	values, ok := redisIntegers(result, 3)
	if !ok {
		return Decision{}, ErrUnavailable
	}

	allowedValue := values[0]
	count := values[1]
	ttlMilliseconds := values[2]

	if (allowedValue != 0 && allowedValue != 1) ||
		count < 1 ||
		ttlMilliseconds < 0 {
		return Decision{}, ErrUnavailable
	}

	remaining := policy.limit - count
	if remaining < 0 {
		remaining = 0
	}

	retryAfter := time.Duration(
		ttlMilliseconds,
	) * time.Millisecond

	if retryAfter < time.Millisecond {
		retryAfter = time.Millisecond
	}

	return Decision{
		Allowed:    allowedValue == 1,
		Remaining:  remaining,
		RetryAfter: retryAfter,
	}, nil
}

type LockoutDecision struct {
	Locked     bool
	Failures   int64
	RetryAfter time.Duration
}

type LoginProtector struct {
	runner  ScriptRunner
	builder *KeyBuilder
	policy  LoginPolicy
}

func NewLoginProtector(
	runner ScriptRunner,
	builder *KeyBuilder,
	policy LoginPolicy,
) (*LoginProtector, error) {
	if runner == nil || builder == nil {
		return nil, ErrInvalidConfiguration
	}

	if err := policy.validate(); err != nil {
		return nil, err
	}

	return &LoginProtector{
		runner:  runner,
		builder: builder,
		policy:  policy,
	}, nil
}

func (protector *LoginProtector) Check(
	ctx context.Context,
	identityParts ...string,
) (LockoutDecision, error) {
	if ctx == nil ||
		protector == nil ||
		protector.runner == nil ||
		protector.builder == nil {
		return LockoutDecision{}, ErrUnavailable
	}

	lockoutKey, err := protector.builder.Key(
		scopeLoginLockout,
		identityParts...,
	)
	if err != nil {
		return LockoutDecision{}, err
	}

	result, err := protector.runner.RunScript(
		ctx,
		checkLockoutScript,
		[]string{lockoutKey},
		protector.policy.lockout.Milliseconds(),
	)
	if err != nil {
		return LockoutDecision{},
			mapRunnerError(err)
	}

	values, ok := redisIntegers(result, 2)
	if !ok {
		return LockoutDecision{}, ErrUnavailable
	}

	lockedValue := values[0]
	ttlMilliseconds := values[1]

	if (lockedValue != 0 && lockedValue != 1) ||
		ttlMilliseconds < 0 {
		return LockoutDecision{}, ErrUnavailable
	}

	return LockoutDecision{
		Locked: lockedValue == 1,
		RetryAfter: durationFromMilliseconds(
			ttlMilliseconds,
		),
	}, nil
}

func (protector *LoginProtector) RecordFailure(
	ctx context.Context,
	identityParts ...string,
) (LockoutDecision, error) {
	if ctx == nil ||
		protector == nil ||
		protector.runner == nil ||
		protector.builder == nil {
		return LockoutDecision{}, ErrUnavailable
	}

	failureKey, err := protector.builder.Key(
		scopeLoginFailure,
		identityParts...,
	)
	if err != nil {
		return LockoutDecision{}, err
	}

	lockoutKey, err := protector.builder.Key(
		scopeLoginLockout,
		identityParts...,
	)
	if err != nil {
		return LockoutDecision{}, err
	}

	result, err := protector.runner.RunScript(
		ctx,
		recordLoginFailureScript,
		[]string{
			failureKey,
			lockoutKey,
		},
		protector.policy.failureLimit,
		protector.policy.failureWindow.Milliseconds(),
		protector.policy.lockout.Milliseconds(),
	)
	if err != nil {
		return LockoutDecision{},
			mapRunnerError(err)
	}

	values, ok := redisIntegers(result, 3)
	if !ok {
		return LockoutDecision{}, ErrUnavailable
	}

	lockedValue := values[0]
	ttlMilliseconds := values[1]
	failures := values[2]

	if (lockedValue != 0 && lockedValue != 1) ||
		ttlMilliseconds < 0 ||
		failures < 1 {
		return LockoutDecision{}, ErrUnavailable
	}

	return LockoutDecision{
		Locked:   lockedValue == 1,
		Failures: failures,
		RetryAfter: durationFromMilliseconds(
			ttlMilliseconds,
		),
	}, nil
}

func (protector *LoginProtector) Clear(
	ctx context.Context,
	identityParts ...string,
) error {
	if ctx == nil ||
		protector == nil ||
		protector.runner == nil ||
		protector.builder == nil {
		return ErrUnavailable
	}

	failureKey, err := protector.builder.Key(
		scopeLoginFailure,
		identityParts...,
	)
	if err != nil {
		return err
	}

	lockoutKey, err := protector.builder.Key(
		scopeLoginLockout,
		identityParts...,
	)
	if err != nil {
		return err
	}

	_, err = protector.runner.RunScript(
		ctx,
		clearLoginProtectionScript,
		[]string{
			failureKey,
			lockoutKey,
		},
	)
	if err != nil {
		return mapRunnerError(err)
	}

	return nil
}

func mapRunnerError(err error) error {
	switch {
	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return ErrUnavailable
	}
}

func durationFromMilliseconds(
	milliseconds int64,
) time.Duration {
	if milliseconds <= 0 {
		return 0
	}

	duration := time.Duration(
		milliseconds,
	) * time.Millisecond

	if duration < time.Millisecond {
		return time.Millisecond
	}

	return duration
}

func redisIntegers(
	result any,
	expectedLength int,
) ([]int64, bool) {
	values, ok := result.([]interface{})
	if !ok || len(values) != expectedLength {
		return nil, false
	}

	integers := make(
		[]int64,
		expectedLength,
	)

	for index, value := range values {
		integer, ok := redisInteger(value)
		if !ok {
			return nil, false
		}

		integers[index] = integer
	}

	return integers, true
}

func redisInteger(value any) (int64, bool) {
	switch typedValue := value.(type) {
	case int64:
		return typedValue, true

	case int:
		return int64(typedValue), true

	case string:
		integer, err := strconv.ParseInt(
			typedValue,
			10,
			64,
		)

		return integer, err == nil

	case []byte:
		integer, err := strconv.ParseInt(
			string(typedValue),
			10,
			64,
		)

		return integer, err == nil

	default:
		return 0, false
	}
}
