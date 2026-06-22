package sessionhandler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/api/sessioncookie"
)

func TestHandlerClearsBrowserSessionAfterCurrentLogout(t *testing.T) {
	t.Parallel()

	service := &handlerTestSessionService{}
	router := newHandlerTestRouter(service)
	request := newAuthenticatedHandlerRequest(http.MethodDelete, "/v1/sessions/current")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertClearingSessionCookies(t, recorder)
}

func TestHandlerClearsBrowserSessionAfterLogoutAll(t *testing.T) {
	t.Parallel()

	service := &handlerTestSessionService{}
	router := newHandlerTestRouter(service)
	request := newAuthenticatedHandlerRequest(http.MethodDelete, "/v1/sessions")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	assertClearingSessionCookies(t, recorder)
}

func TestHandlerClearsBrowserSessionOnlyWhenRevokingCurrentSession(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		sessionID   string
		wantCookies bool
	}{
		{name: "current session", sessionID: handlerTestCurrentSessionID, wantCookies: true},
		{name: "other session", sessionID: handlerTestOtherSessionID, wantCookies: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			service := &handlerTestSessionService{}
			router := newHandlerTestRouter(service)
			request := newAuthenticatedHandlerRequest(
				http.MethodDelete,
				"/v1/sessions/"+testCase.sessionID,
			)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}
			if testCase.wantCookies {
				assertClearingSessionCookies(t, recorder)
				return
			}
			if len(recorder.Result().Cookies()) != 0 {
				t.Fatal("revoking another session cleared the current browser cookies")
			}
		})
	}
}

func assertClearingSessionCookies(t *testing.T, recorder *httptest.ResponseRecorder) {
	t.Helper()

	config := sessioncookie.NewConfig(false)
	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}

	for _, name := range []string{config.RefreshCookieName(), config.CSRFCookieName()} {
		cookie := sessionHandlerTestCookie(t, cookies, name)
		if cookie.Value != "" || cookie.MaxAge != -1 {
			t.Fatalf("cookie %q was not cleared", name)
		}
	}
}

func sessionHandlerTestCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q was not found", name)
	return nil
}
