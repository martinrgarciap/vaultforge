package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func FuzzBearerToken(f *testing.F) {
	f.Add("Bearer synthetic-token")
	f.Add("bearer another.synthetic.token")
	f.Add("Basic credentials")
	f.Add("")

	f.Fuzz(func(t *testing.T, authorization string) {
		request := httptest.NewRequest("GET", "/v1/vaults", nil)
		request.Header.Set("Authorization", authorization)

		tokenValue, ok := bearerToken(request)
		if !ok {
			return
		}

		if tokenValue == "" || len(tokenValue) > maxBearerTokenBytes {
			t.Fatal("accepted bearer token violated length bounds")
		}

		if strings.ContainsAny(tokenValue, " \t\r\n") {
			t.Fatal("accepted bearer token contained whitespace")
		}

		replayed := httptest.NewRequest("GET", "/v1/vaults", nil)
		replayed.Header.Set("Authorization", "Bearer "+tokenValue)

		replayedToken, replayedOK := bearerToken(replayed)
		if !replayedOK || replayedToken != tokenValue {
			t.Fatal("accepted bearer token was not stable")
		}
	})
}
