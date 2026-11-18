package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestGoogleSignInRedirectsAndChecksState(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "cid.apps.googleusercontent.com")
	t.Setenv("GOOGLE_CLIENT_SECRET", "secret")
	t.Setenv("APP_BASE_URL", "https://apex.test")
	_, h := newTestAPI(t)

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/providers", nil))
	if !strings.Contains(w.Body.String(), `"google":true`) {
		t.Fatalf("providers = %s", w.Body)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/google/start?next=/workflows", nil))
	expect(t, w, http.StatusFound, "start")
	loc, _ := url.Parse(w.Header().Get("Location"))
	q := loc.Query()
	if loc.Host != "accounts.google.com" || q.Get("redirect_uri") != "https://apex.test/api/auth/google/callback" || q.Get("scope") != "openid email profile" {
		t.Fatalf("redirect = %s", loc)
	}
	var state *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == googleStateCookie {
			state = c
		}
	}
	if state == nil || !state.HttpOnly || !strings.HasPrefix(state.Value, q.Get("state")+"|") {
		t.Fatalf("state cookie = %+v", state)
	}

	// A callback whose state doesn't match the cookie is refused.
	req := httptest.NewRequest("GET", "/api/auth/google/callback?state=forged&code=x", nil)
	req.AddCookie(state)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/signin?error=google" {
		t.Fatalf("forged state: %d %s", w.Code, w.Header().Get("Location"))
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge > 0 {
			t.Fatal("forged state got a session")
		}
	}
}

func TestGoogleStartWithoutConfigGoesBackToSignIn(t *testing.T) {
	t.Setenv("GOOGLE_CLIENT_ID", "")
	t.Setenv("GOOGLE_CLIENT_SECRET", "")
	_, h := newTestAPI(t)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/auth/google/start", nil))
	if w.Code != http.StatusFound || w.Header().Get("Location") != "/signin?error=google" {
		t.Fatalf("got %d %s", w.Code, w.Header().Get("Location"))
	}
}

func TestConnectRoutesNeedASession(t *testing.T) {
	_, h := newTestAPI(t)
	c := &apiClient{t: t, h: h}
	for _, path := range []string{"/api/oauth/apps", "/api/oauth/pieces.gmail/start", "/api/oauth/callback", "/api/connect/telegram/chats"} {
		expect(t, c.do("GET", path, ""), http.StatusUnauthorized, path)
	}
	expect(t, c.do("POST", "/api/connect/telegram/start", `{}`), http.StatusUnauthorized, "telegram start")
}

func TestConnectResultPageEscapes(t *testing.T) {
	w := httptest.NewRecorder()
	connectResultPage(w, map[string]any{"ok": false, "error": `</script><script>alert(1)</script>`})
	body := w.Body.String()
	if strings.Contains(body, "</script><script>alert(1)") {
		t.Fatalf("unescaped: %s", body)
	}
	if !strings.Contains(body, "postMessage(r,window.location.origin)") {
		t.Fatal("result must only be posted to this origin")
	}
}
