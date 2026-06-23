package buildinfo

import "strings"

const (
	ServiceName          = "vaultforge-api"
	defaultVersion       = "development"
	defaultCommit        = "unknown"
	maxBuildMetadataSize = 128
)

var (
	Version = defaultVersion
	Commit  = defaultCommit
)

type Info struct {
	Service string `json:"service"`
	Version string `json:"version"`
	Commit  string `json:"commit"`
}

func Current() Info {
	return New(Version, Commit)
}

func New(version string, commit string) Info {
	return Info{
		Service: ServiceName,
		Version: safeVersion(version),
		Commit:  safeCommit(commit),
	}
}

func safeVersion(value string) string {
	value = strings.TrimSpace(value)

	if value == "" || len(value) > maxBuildMetadataSize {
		return defaultVersion
	}

	for index := 0; index < len(value); index++ {
		if !validVersionCharacter(value[index]) {
			return defaultVersion
		}
	}

	return value
}

func safeCommit(value string) string {
	value = strings.TrimSpace(value)

	if value == defaultCommit {
		return value
	}

	if len(value) < 7 || len(value) > 64 {
		return defaultCommit
	}

	for index := 0; index < len(value); index++ {
		character := value[index]

		if !((character >= '0' && character <= '9') ||
			(character >= 'a' && character <= 'f') ||
			(character >= 'A' && character <= 'F')) {
			return defaultCommit
		}
	}

	return strings.ToLower(value)
}

func validVersionCharacter(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z':
		return true
	case character >= 'A' && character <= 'Z':
		return true
	case character >= '0' && character <= '9':
		return true
	}

	switch character {
	case '.', '-', '_', '+':
		return true
	default:
		return false
	}
}
