package ratelimit

import "context"

type Enforcer struct {
	limiter        *Limiter
	loginProtector *LoginProtector
}

func NewEnforcer(
	runner ScriptRunner,
	builder *KeyBuilder,
	loginPolicy LoginPolicy,
) (*Enforcer, error) {
	limiter, err := NewLimiter(
		runner,
		builder,
	)
	if err != nil {
		return nil, err
	}

	loginProtector, err := NewLoginProtector(
		runner,
		builder,
		loginPolicy,
	)
	if err != nil {
		return nil, err
	}

	return &Enforcer{
		limiter:        limiter,
		loginProtector: loginProtector,
	}, nil
}

func (enforcer *Enforcer) Allow(
	ctx context.Context,
	scope string,
	policy Policy,
	identityParts ...string,
) (Decision, error) {
	if enforcer == nil || enforcer.limiter == nil {
		return Decision{}, ErrUnavailable
	}

	return enforcer.limiter.Allow(
		ctx,
		scope,
		policy,
		identityParts...,
	)
}

func (enforcer *Enforcer) Check(
	ctx context.Context,
	identityParts ...string,
) (LockoutDecision, error) {
	if enforcer == nil ||
		enforcer.loginProtector == nil {
		return LockoutDecision{}, ErrUnavailable
	}

	return enforcer.loginProtector.Check(
		ctx,
		identityParts...,
	)
}

func (enforcer *Enforcer) RecordFailure(
	ctx context.Context,
	identityParts ...string,
) (LockoutDecision, error) {
	if enforcer == nil ||
		enforcer.loginProtector == nil {
		return LockoutDecision{}, ErrUnavailable
	}

	return enforcer.loginProtector.RecordFailure(
		ctx,
		identityParts...,
	)
}

func (enforcer *Enforcer) Clear(
	ctx context.Context,
	identityParts ...string,
) error {
	if enforcer == nil ||
		enforcer.loginProtector == nil {
		return ErrUnavailable
	}

	return enforcer.loginProtector.Clear(
		ctx,
		identityParts...,
	)
}
