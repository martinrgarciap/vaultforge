package ratelimit

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeScriptRunner struct {
	result any
	err    error
	keys   []string
}

func (runner *fakeScriptRunner) RunScript(
	_ context.Context,
	_ string,
	keys []string,
	_ ...any,
) (any, error) {
	runner.keys = append(
		[]string(nil),
		keys...,
	)

	return runner.result, runner.err
}

func newTestKeyBuilder(
	t *testing.T,
) *KeyBuilder {
	t.Helper()

	builder, err := NewKeyBuilder(
		"vaultforge:test:v1",
		bytes.Repeat(
			[]byte{0x42},
			MinIdentityHMACKeyBytes,
		),
	)
	if err != nil {
		t.Fatalf(
			"create key builder: %v",
			err,
		)
	}

	return builder
}

func TestKeyBuilderProducesStableScopedOpaqueKeys(
	t *testing.T,
) {
	builder := newTestKeyBuilder(t)

	const (
		emailAddress = "person@example.test"
		ipAddress    = "203.0.113.10"
	)

	first, err := builder.Key(
		ScopeLogin,
		emailAddress,
		ipAddress,
	)
	if err != nil {
		t.Fatalf("build first key: %v", err)
	}

	second, err := builder.Key(
		ScopeLogin,
		emailAddress,
		ipAddress,
	)
	if err != nil {
		t.Fatalf("build second key: %v", err)
	}

	differentScope, err := builder.Key(
		ScopeRegistration,
		emailAddress,
		ipAddress,
	)
	if err != nil {
		t.Fatalf(
			"build differently scoped key: %v",
			err,
		)
	}

	if first != second {
		t.Fatal("identical identities produced different keys")
	}

	if first == differentScope {
		t.Fatal("different scopes produced the same key")
	}

	for _, forbidden := range []string{
		emailAddress,
		ipAddress,
	} {
		if strings.Contains(first, forbidden) {
			t.Fatalf(
				"key exposed raw identity %q",
				forbidden,
			)
		}
	}
}

func TestNewKeyBuilderRejectsShortKey(
	t *testing.T,
) {
	_, err := NewKeyBuilder(
		"vaultforge:test:v1",
		[]byte("too-short"),
	)
	if err == nil {
		t.Fatal("expected short HMAC key to fail")
	}

	if !errors.Is(
		err,
		ErrInvalidConfiguration,
	) {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestLimiterReturnsParsedDecision(
	t *testing.T,
) {
	runner := &fakeScriptRunner{
		result: []interface{}{
			int64(0),
			int64(3),
			int64(1250),
		},
	}

	limiter, err := NewLimiter(
		runner,
		newTestKeyBuilder(t),
	)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	policy, err := NewPolicy(
		2,
		2*time.Second,
	)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	decision, err := limiter.Allow(
		context.Background(),
		ScopeLogin,
		policy,
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf("allow request: %v", err)
	}

	if decision.Allowed {
		t.Fatal("expected request to be denied")
	}

	if decision.Remaining != 0 {
		t.Fatalf(
			"remaining = %d, want 0",
			decision.Remaining,
		)
	}

	if decision.RetryAfter !=
		1250*time.Millisecond {
		t.Fatalf(
			"retry after = %v",
			decision.RetryAfter,
		)
	}
}

func TestLimiterMapsRunnerFailureSafely(
	t *testing.T,
) {
	policy, err := NewPolicy(
		2,
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	testCases := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "dependency error",
			err: errors.New(
				"synthetic connection details",
			),
			want: ErrUnavailable,
		},
		{
			name: "canceled context",
			err:  context.Canceled,
			want: context.Canceled,
		},
		{
			name: "deadline exceeded",
			err:  context.DeadlineExceeded,
			want: context.DeadlineExceeded,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := &fakeScriptRunner{
				err: testCase.err,
			}

			limiter, err := NewLimiter(
				runner,
				newTestKeyBuilder(t),
			)
			if err != nil {
				t.Fatalf(
					"create limiter: %v",
					err,
				)
			}

			_, err = limiter.Allow(
				context.Background(),
				ScopeLogin,
				policy,
				"203.0.113.10",
			)
			if !errors.Is(err, testCase.want) {
				t.Fatalf(
					"error = %v, want %v",
					err,
					testCase.want,
				)
			}
		})
	}
}

func TestLoginProtectorUsesOpaqueKeys(
	t *testing.T,
) {
	runner := &fakeScriptRunner{
		result: []interface{}{
			int64(0),
			int64(60_000),
			int64(1),
		},
	}

	policy, err := NewLoginPolicy(
		5,
		15*time.Minute,
		15*time.Minute,
	)
	if err != nil {
		t.Fatalf(
			"create login policy: %v",
			err,
		)
	}

	protector, err := NewLoginProtector(
		runner,
		newTestKeyBuilder(t),
		policy,
	)
	if err != nil {
		t.Fatalf(
			"create login protector: %v",
			err,
		)
	}

	const (
		emailAddress = "person@example.test"
		ipAddress    = "203.0.113.10"
	)

	_, err = protector.RecordFailure(
		context.Background(),
		emailAddress,
		ipAddress,
	)
	if err != nil {
		t.Fatalf(
			"record login failure: %v",
			err,
		)
	}

	if len(runner.keys) != 2 {
		t.Fatalf(
			"key count = %d, want 2",
			len(runner.keys),
		)
	}

	for _, key := range runner.keys {
		for _, forbidden := range []string{
			emailAddress,
			ipAddress,
		} {
			if strings.Contains(key, forbidden) {
				t.Fatalf(
					"Redis key exposed %q",
					forbidden,
				)
			}
		}
	}
}
