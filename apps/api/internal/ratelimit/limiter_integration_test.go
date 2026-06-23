package ratelimit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/redisclient"
	"github.com/redis/go-redis/v9"
)

type redisIntegration struct {
	runner    *redisclient.Client
	inspector *redis.Client
	builder   *KeyBuilder
	prefix    string
}

func newRedisIntegration(
	t *testing.T,
) *redisIntegration {
	t.Helper()

	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL is not configured")
	}

	config, err := redisclient.NewConfig(
		redisURL,
		redisclient.DefaultDialTimeout,
		redisclient.DefaultReadTimeout,
		redisclient.DefaultWriteTimeout,
		redisclient.DefaultPoolTimeout,
	)
	if err != nil {
		t.Fatalf(
			"create Redis configuration: %v",
			err,
		)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	runner, err := redisclient.New(
		ctx,
		config,
	)
	if err != nil {
		t.Fatalf(
			"create Redis client: %v",
			err,
		)
	}

	options, err := redis.ParseURL(redisURL)
	if err != nil {
		_ = runner.Close()

		t.Fatal("parse test Redis URL")
	}

	inspector := redis.NewClient(options)

	if err := inspector.Ping(ctx).Err(); err != nil {
		_ = inspector.Close()
		_ = runner.Close()

		t.Fatal("ping test Redis")
	}

	prefix := fmt.Sprintf(
		"vaultforge:test:%d",
		time.Now().UnixNano(),
	)

	builder, err := NewKeyBuilder(
		prefix,
		bytes.Repeat(
			[]byte{0x42},
			MinIdentityHMACKeyBytes,
		),
	)
	if err != nil {
		_ = inspector.Close()
		_ = runner.Close()

		t.Fatalf(
			"create key builder: %v",
			err,
		)
	}

	integration := &redisIntegration{
		runner:    runner,
		inspector: inspector,
		builder:   builder,
		prefix:    prefix,
	}

	t.Cleanup(func() {
		cleanupContext, cleanupCancel :=
			context.WithTimeout(
				context.Background(),
				5*time.Second,
			)
		defer cleanupCancel()

		integration.deleteKeys(
			cleanupContext,
		)

		_ = inspector.Close()
		_ = runner.Close()
	})

	return integration
}

func (integration *redisIntegration) keys(
	ctx context.Context,
	t *testing.T,
) []string {
	t.Helper()

	var (
		cursor uint64
		keys   []string
	)

	for {
		found, nextCursor, err :=
			integration.inspector.Scan(
				ctx,
				cursor,
				integration.prefix+":*",
				100,
			).Result()
		if err != nil {
			t.Fatalf(
				"scan Redis test keys: %v",
				err,
			)
		}

		keys = append(keys, found...)
		cursor = nextCursor

		if cursor == 0 {
			return keys
		}
	}
}

func (integration *redisIntegration) deleteKeys(
	ctx context.Context,
) {
	var cursor uint64

	for {
		keys, nextCursor, err :=
			integration.inspector.Scan(
				ctx,
				cursor,
				integration.prefix+":*",
				100,
			).Result()
		if err != nil {
			return
		}

		if len(keys) > 0 {
			_ = integration.inspector.Del(
				ctx,
				keys...,
			).Err()
		}

		cursor = nextCursor

		if cursor == 0 {
			return
		}
	}
}

func TestLimiterEnforcesAtomicFixedWindow(
	t *testing.T,
) {
	integration := newRedisIntegration(t)

	limiter, err := NewLimiter(
		integration.runner,
		integration.builder,
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

	ctx := context.Background()

	for request := 1; request <= 2; request++ {
		decision, err := limiter.Allow(
			ctx,
			ScopeRegistration,
			policy,
			"203.0.113.10",
		)
		if err != nil {
			t.Fatalf(
				"allow request %d: %v",
				request,
				err,
			)
		}

		if !decision.Allowed {
			t.Fatalf(
				"request %d was unexpectedly denied",
				request,
			)
		}
	}

	decision, err := limiter.Allow(
		ctx,
		ScopeRegistration,
		policy,
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf(
			"allow third request: %v",
			err,
		)
	}

	if decision.Allowed {
		t.Fatal("third request was unexpectedly allowed")
	}

	if decision.RetryAfter <= 0 ||
		decision.RetryAfter > policy.window {
		t.Fatalf(
			"retry after = %v",
			decision.RetryAfter,
		)
	}

	keys := integration.keys(ctx, t)
	if len(keys) != 1 {
		t.Fatalf(
			"Redis key count = %d, want 1",
			len(keys),
		)
	}

	ttl, err := integration.inspector.PTTL(
		ctx,
		keys[0],
	).Result()
	if err != nil {
		t.Fatalf("read counter TTL: %v", err)
	}

	if ttl <= 0 || ttl > policy.window {
		t.Fatalf(
			"counter TTL = %v",
			ttl,
		)
	}
}

func TestLimiterSeparatesIdentities(
	t *testing.T,
) {
	integration := newRedisIntegration(t)

	limiter, err := NewLimiter(
		integration.runner,
		integration.builder,
	)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	policy, err := NewPolicy(
		1,
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	first, err := limiter.Allow(
		context.Background(),
		ScopeLogin,
		policy,
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf(
			"allow first identity: %v",
			err,
		)
	}

	second, err := limiter.Allow(
		context.Background(),
		ScopeLogin,
		policy,
		"203.0.113.11",
	)
	if err != nil {
		t.Fatalf(
			"allow second identity: %v",
			err,
		)
	}

	if !first.Allowed || !second.Allowed {
		t.Fatal(
			"separate identities incorrectly shared a counter",
		)
	}
}

func TestLimiterIsAtomicUnderConcurrency(
	t *testing.T,
) {
	integration := newRedisIntegration(t)

	limiter, err := NewLimiter(
		integration.runner,
		integration.builder,
	)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	const (
		limit    = 10
		requests = 50
	)

	policy, err := NewPolicy(
		limit,
		time.Minute,
	)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	var (
		allowed atomic.Int64
		wait    sync.WaitGroup
	)

	errorsChannel := make(
		chan error,
		requests,
	)

	for request := 0; request < requests; request++ {
		wait.Add(1)

		go func() {
			defer wait.Done()

			decision, err := limiter.Allow(
				context.Background(),
				ScopeMutation,
				policy,
				"synthetic-user-id",
			)
			if err != nil {
				errorsChannel <- err

				return
			}

			if decision.Allowed {
				allowed.Add(1)
			}
		}()
	}

	wait.Wait()
	close(errorsChannel)

	for err := range errorsChannel {
		t.Fatalf(
			"concurrent limiter request: %v",
			err,
		)
	}

	if allowed.Load() != limit {
		t.Fatalf(
			"allowed requests = %d, want %d",
			allowed.Load(),
			limit,
		)
	}
}

func TestLimiterWindowExpires(
	t *testing.T,
) {
	integration := newRedisIntegration(t)

	limiter, err := NewLimiter(
		integration.runner,
		integration.builder,
	)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	policy, err := NewPolicy(
		1,
		250*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	ctx := context.Background()

	first, err := limiter.Allow(
		ctx,
		ScopeRefresh,
		policy,
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf("allow first request: %v", err)
	}

	second, err := limiter.Allow(
		ctx,
		ScopeRefresh,
		policy,
		"203.0.113.10",
	)
	if err != nil {
		t.Fatalf("allow second request: %v", err)
	}

	if !first.Allowed || second.Allowed {
		t.Fatal(
			"initial fixed-window behavior was incorrect",
		)
	}

	deadline := time.Now().Add(
		3 * time.Second,
	)

	for time.Now().Before(deadline) {
		decision, err := limiter.Allow(
			ctx,
			ScopeRefresh,
			policy,
			"203.0.113.10",
		)
		if err != nil {
			t.Fatalf(
				"check expired window: %v",
				err,
			)
		}

		if decision.Allowed {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("fixed window did not expire")
}

func TestLoginProtectorLocksExpiresAndClears(
	t *testing.T,
) {
	integration := newRedisIntegration(t)

	policy, err := NewLoginPolicy(
		3,
		2*time.Second,
		300*time.Millisecond,
	)
	if err != nil {
		t.Fatalf(
			"create login policy: %v",
			err,
		)
	}

	protector, err := NewLoginProtector(
		integration.runner,
		integration.builder,
		policy,
	)
	if err != nil {
		t.Fatalf(
			"create login protector: %v",
			err,
		)
	}

	ctx := context.Background()
	identity := []string{
		"person@example.test",
		"203.0.113.10",
	}

	for failure := 1; failure <= 2; failure++ {
		decision, err := protector.RecordFailure(
			ctx,
			identity...,
		)
		if err != nil {
			t.Fatalf(
				"record failure %d: %v",
				failure,
				err,
			)
		}

		if decision.Locked {
			t.Fatalf(
				"failure %d locked too early",
				failure,
			)
		}

		if decision.Failures != int64(failure) {
			t.Fatalf(
				"failure count = %d, want %d",
				decision.Failures,
				failure,
			)
		}
	}

	locked, err := protector.RecordFailure(
		ctx,
		identity...,
	)
	if err != nil {
		t.Fatalf(
			"record locking failure: %v",
			err,
		)
	}

	if !locked.Locked ||
		locked.RetryAfter <= 0 {
		t.Fatal(
			"threshold failure did not activate lockout",
		)
	}

	checked, err := protector.Check(
		ctx,
		identity...,
	)
	if err != nil {
		t.Fatalf("check lockout: %v", err)
	}

	if !checked.Locked {
		t.Fatal("active lockout was not detected")
	}

	deadline := time.Now().Add(
		3 * time.Second,
	)

	for time.Now().Before(deadline) {
		checked, err = protector.Check(
			ctx,
			identity...,
		)
		if err != nil {
			t.Fatalf(
				"check lockout expiration: %v",
				err,
			)
		}

		if !checked.Locked {
			break
		}

		time.Sleep(20 * time.Millisecond)
	}

	if checked.Locked {
		t.Fatal("login lockout did not expire")
	}

	firstAfterExpiration, err :=
		protector.RecordFailure(
			ctx,
			identity...,
		)
	if err != nil {
		t.Fatalf(
			"record failure after expiration: %v",
			err,
		)
	}

	if firstAfterExpiration.Failures != 1 {
		t.Fatalf(
			"failure count after expiration = %d, want 1",
			firstAfterExpiration.Failures,
		)
	}

	if err := protector.Clear(
		ctx,
		identity...,
	); err != nil {
		t.Fatalf(
			"clear login protection: %v",
			err,
		)
	}

	firstAfterClear, err :=
		protector.RecordFailure(
			ctx,
			identity...,
		)
	if err != nil {
		t.Fatalf(
			"record failure after clear: %v",
			err,
		)
	}

	if firstAfterClear.Failures != 1 {
		t.Fatalf(
			"failure count after clear = %d, want 1",
			firstAfterClear.Failures,
		)
	}
}

func TestRedisKeysAndValuesDoNotContainRawIdentities(
	t *testing.T,
) {
	integration := newRedisIntegration(t)

	limiter, err := NewLimiter(
		integration.runner,
		integration.builder,
	)
	if err != nil {
		t.Fatalf("create limiter: %v", err)
	}

	loginPolicy, err := NewLoginPolicy(
		5,
		time.Minute,
		time.Minute,
	)
	if err != nil {
		t.Fatalf(
			"create login policy: %v",
			err,
		)
	}

	protector, err := NewLoginProtector(
		integration.runner,
		integration.builder,
		loginPolicy,
	)
	if err != nil {
		t.Fatalf(
			"create login protector: %v",
			err,
		)
	}

	ratePolicy, err := NewPolicy(
		5,
		time.Minute,
	)
	if err != nil {
		t.Fatalf(
			"create rate policy: %v",
			err,
		)
	}

	const (
		emailAddress = "person@example.test"
		ipAddress    = "203.0.113.10"
		secretMarker = "synthetic-secret-marker"
	)

	ctx := context.Background()

	if _, err := limiter.Allow(
		ctx,
		ScopeLogin,
		ratePolicy,
		ipAddress,
		secretMarker,
	); err != nil {
		t.Fatalf(
			"create rate-limit key: %v",
			err,
		)
	}

	if _, err := protector.RecordFailure(
		ctx,
		emailAddress,
		ipAddress,
	); err != nil {
		t.Fatalf(
			"create login-failure key: %v",
			err,
		)
	}

	forbiddenValues := []string{
		emailAddress,
		ipAddress,
		secretMarker,
	}

	keys := integration.keys(ctx, t)

	if len(keys) == 0 {
		t.Fatal("expected Redis keys")
	}

	for _, key := range keys {
		for _, forbidden := range forbiddenValues {
			if strings.Contains(key, forbidden) {
				t.Fatalf(
					"Redis key exposed %q",
					forbidden,
				)
			}
		}

		value, err := integration.inspector.Get(
			ctx,
			key,
		).Result()
		if err != nil {
			t.Fatalf(
				"read Redis value: %v",
				err,
			)
		}

		for _, forbidden := range forbiddenValues {
			if strings.Contains(value, forbidden) {
				t.Fatalf(
					"Redis value exposed %q",
					forbidden,
				)
			}
		}
	}
}
