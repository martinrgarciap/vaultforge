package health

import (
	"context"
	"errors"
	"testing"
)

type fakeReadinessDependency struct {
	err   error
	calls int
}

func (dependency *fakeReadinessDependency) Ping(_ context.Context) error {
	dependency.calls++

	return dependency.err
}

func TestReadinessPingerChecksEveryDependency(t *testing.T) {
	database := &fakeReadinessDependency{}
	redis := &fakeReadinessDependency{}

	pinger := NewReadinessPinger(database, redis)

	if err := pinger.Ping(context.Background()); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if database.calls != 1 {
		t.Errorf("database calls = %d, want 1", database.calls)
	}

	if redis.calls != 1 {
		t.Errorf("Redis calls = %d, want 1", redis.calls)
	}
}

func TestReadinessPingerFailsClosedWhenDependencyFails(t *testing.T) {
	database := &fakeReadinessDependency{}
	redis := &fakeReadinessDependency{
		err: errors.New("redis connection failed with secret details"),
	}

	pinger := NewReadinessPinger(database, redis)

	err := pinger.Ping(context.Background())
	if err == nil {
		t.Fatal("expected dependency failure to return an error")
	}

	if err.Error() != "readiness dependency unavailable" {
		t.Fatalf("unexpected safe error: %q", err.Error())
	}

	if database.calls != 1 {
		t.Errorf("database calls = %d, want 1", database.calls)
	}

	if redis.calls != 1 {
		t.Errorf("Redis calls = %d, want 1", redis.calls)
	}
}

func TestReadinessPingerRejectsMissingDependencies(t *testing.T) {
	testCases := []struct {
		name   string
		pinger *ReadinessPinger
	}{
		{
			name:   "no dependencies",
			pinger: NewReadinessPinger(),
		},
		{
			name:   "nil dependency",
			pinger: NewReadinessPinger(nil),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := testCase.pinger.Ping(context.Background()); err == nil {
				t.Fatal("expected missing dependency to return an error")
			}
		})
	}
}
