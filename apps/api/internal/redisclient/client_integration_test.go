package redisclient

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestClientConnectsAndPingsRedis(t *testing.T) {
	redisURL := os.Getenv("TEST_REDIS_URL")
	if redisURL == "" {
		t.Skip("TEST_REDIS_URL is not configured")
	}

	config, err := NewConfig(
		redisURL,
		DefaultDialTimeout,
		DefaultReadTimeout,
		DefaultWriteTimeout,
		DefaultPoolTimeout,
	)
	if err != nil {
		t.Fatalf("create Redis configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := New(ctx, config)
	if err != nil {
		t.Fatalf("connect to Redis: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			t.Errorf("close Redis client: %v", err)
		}
	}()

	if err := client.Ping(ctx); err != nil {
		t.Fatalf("ping Redis: %v", err)
	}
}

func TestClientFailsSafelyWhenRedisIsUnavailable(t *testing.T) {
	config, err := NewConfig(
		"redis://127.0.0.1:1/0",
		100*time.Millisecond,
		100*time.Millisecond,
		100*time.Millisecond,
		100*time.Millisecond,
	)
	if err != nil {
		t.Fatalf("create Redis configuration: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err = New(ctx, config)
	if err == nil {
		t.Fatal("expected unavailable Redis to return an error")
	}

	if err.Error() != "redis unavailable" {
		t.Fatalf("unexpected public-safe error: %q", err.Error())
	}
}

func TestClientDoesNotLogConnectionDetailsWhenUnavailable(t *testing.T) {
	const helperEnvironment = "VAULTFORGE_REDIS_LOG_HELPER"

	if os.Getenv(helperEnvironment) == "1" {
		config, err := NewConfig(
			"redis://synthetic-user:synthetic-password@127.0.0.1:1/0",
			100*time.Millisecond,
			100*time.Millisecond,
			100*time.Millisecond,
			100*time.Millisecond,
		)
		if err != nil {
			t.Fatalf("create Redis configuration: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		_, err = New(ctx, config)
		if err == nil {
			t.Fatal("expected unavailable Redis to return an error")
		}

		if err.Error() != "redis unavailable" {
			t.Fatalf("unexpected public-safe error: %q", err.Error())
		}

		return
	}

	command := exec.Command(
		os.Args[0],
		"-test.run=^TestClientDoesNotLogConnectionDetailsWhenUnavailable$",
	)
	command.Env = append(
		os.Environ(),
		helperEnvironment+"=1",
	)

	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf(
			"helper process failed: %v\n%s",
			err,
			output,
		)
	}

	forbiddenValues := []string{
		"synthetic-user",
		"synthetic-password",
		"127.0.0.1:1",
		"dial tcp",
		"connection refused",
	}

	for _, forbiddenValue := range forbiddenValues {
		if strings.Contains(string(output), forbiddenValue) {
			t.Fatalf(
				"Redis process output exposed %q:\n%s",
				forbiddenValue,
				output,
			)
		}
	}
}
