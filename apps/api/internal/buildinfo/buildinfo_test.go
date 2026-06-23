package buildinfo

import (
	"strings"
	"testing"
)

func TestNewAcceptsSafeBuildMetadata(t *testing.T) {
	t.Parallel()

	info := New("v1.2.3-rc.1", "ABCDEF123456")

	if info.Service != ServiceName {
		t.Fatalf("service = %q, want %q", info.Service, ServiceName)
	}

	if info.Version != "v1.2.3-rc.1" {
		t.Fatalf("version = %q", info.Version)
	}

	if info.Commit != "abcdef123456" {
		t.Fatalf("commit = %q", info.Commit)
	}
}

func TestNewSanitizesUnsafeBuildMetadata(t *testing.T) {
	t.Parallel()

	const secretMarker = "synthetic-secret-marker"

	info := New("release "+secretMarker, "abcdef\n"+secretMarker)

	if info.Version != defaultVersion {
		t.Fatalf("version = %q, want %q", info.Version, defaultVersion)
	}

	if info.Commit != defaultCommit {
		t.Fatalf("commit = %q, want %q", info.Commit, defaultCommit)
	}

	combined := info.Service + info.Version + info.Commit

	if strings.Contains(combined, secretMarker) {
		t.Fatal("sanitized build metadata exposed unsafe input")
	}
}

func TestNewRejectsOversizedBuildMetadata(t *testing.T) {
	t.Parallel()

	info := New(strings.Repeat("a", maxBuildMetadataSize+1), strings.Repeat("a", 65))

	if info.Version != defaultVersion {
		t.Fatal("oversized version was accepted")
	}

	if info.Commit != defaultCommit {
		t.Fatal("oversized commit was accepted")
	}
}
