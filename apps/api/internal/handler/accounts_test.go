package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/services"
)

type apiClient struct {
	t      *testing.T
	h      http.Handler
	cookie *http.Cookie
}

func (c *apiClient) do(method, path, body string) *httptest.ResponseRecorder {
	c.t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "203.0.113.9:4000"
	if c.cookie != nil {
		req.AddCookie(c.cookie)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, req)
	for _, ck := range w.Result().Cookies() {
		if ck.Name == sessionCookie {
			if ck.MaxAge < 0 {
				c.cookie = nil
			} else {
				c.cookie = ck
			}
		}
	}
	return w
}

func expect(t *testing.T, w *httptest.ResponseRecorder, status int, what string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("%s: got %d, want %d: %s", what, w.Code, status, w.Body)
	}
}

func newTestAPI(t *testing.T) (*services.Platform, http.Handler) {
	t.Setenv("AGENTLY_API_TOKEN", "")
	t.Setenv("RUN_CREDENTIAL_CHECK", "off")
	t.Setenv("RUN_LIMIT_PER_DAY", "2")
	p := services.NewPlatform(services.Options{})
	return p, NewRouter(p)
}

func TestSignUpSignInSignOut(t *testing.T) {
	_, h := newTestAPI(t)
	c := &apiClient{t: t, h: h}

	expect(t, c.do("GET", "/api/workflows", ""), http.StatusUnauthorized, "console without a session")

	expect(t, c.do("POST", "/api/auth/signup", `{"email":"not-an-email","password":"longenough"}`), http.StatusBadRequest, "bad email")
	expect(t, c.do("POST", "/api/auth/signup", `{"email":"a@test.dev","password":"short"}`), http.StatusBadRequest, "short password")

	w := c.do("POST", "/api/auth/signup", `{"name":"Ada Lovelace","email":"Ada@Test.dev","password":"correct horse"}`)
	expect(t, w, http.StatusCreated, "sign up")
	if c.cookie == nil || !c.cookie.HttpOnly {
		t.Fatal("sign-up should set an HttpOnly session cookie")
	}
	expect(t, c.do("GET", "/api/workflows", ""), http.StatusOK, "console with a session")

	var me struct{ User *accountUser }
	_ = json.Unmarshal(c.do("GET", "/api/auth/me", "").Body.Bytes(), &me)
	if me.User == nil || me.User.Email != "ada@test.dev" || me.User.Initials != "AL" {
		t.Fatalf("me: %+v", me.User)
	}

	other := &apiClient{t: t, h: h}
	expect(t, other.do("POST", "/api/auth/signup", `{"email":"ada@test.dev","password":"something else"}`), http.StatusConflict, "duplicate email")

	expect(t, c.do("POST", "/api/auth/signout", ""), http.StatusNoContent, "sign out")
	expect(t, c.do("GET", "/api/workflows", ""), http.StatusUnauthorized, "console after sign out")

	expect(t, c.do("POST", "/api/auth/signin", `{"email":"ada@test.dev","password":"wrong password"}`), http.StatusUnauthorized, "wrong password")
	expect(t, c.do("POST", "/api/auth/signin", `{"email":"nobody@test.dev","password":"whatever1"}`), http.StatusUnauthorized, "unknown email")
	expect(t, c.do("POST", "/api/auth/signin", `{"email":"ADA@test.dev","password":"correct horse"}`), http.StatusOK, "sign in")
	expect(t, c.do("GET", "/api/workflows", ""), http.StatusOK, "console after sign in")
}

func TestPublicRoutesStayOpen(t *testing.T) {
	for _, path := range []string{"/api/health", "/api/public/compile", "/api/hooks/x/y", "/api/auth/me"} {
		if !authExempt(path) {
			t.Errorf("%s should not need a session", path)
		}
	}
}

func TestRunsNeedAccountAndRespectOwnership(t *testing.T) {
	p, h := newTestAPI(t)
	bob := &apiClient{t: t, h: h}
	expect(t, bob.do("POST", "/api/workflows/ai-digest/runs", `{}`), http.StatusUnauthorized, "run while signed out")
	expect(t, bob.do("POST", "/api/auth/signup", `{"email":"bob@test.dev","password":"bob password"}`), http.StatusCreated, "bob signs up")

	// ai-digest is an example no account owns: anyone signed in may run it,
	// up to the daily limit, but nobody may change it.
	expect(t, bob.do("POST", "/api/workflows/ai-digest/runs", `{}`), http.StatusCreated, "run an example")
	expect(t, bob.do("POST", "/api/workflows/ai-digest/runs", `{}`), http.StatusCreated, "second run")
	w := bob.do("POST", "/api/workflows/ai-digest/runs", `{}`)
	expect(t, w, http.StatusTooManyRequests, "over the run limit")
	if !strings.Contains(w.Body.String(), "all 2 runs") {
		t.Fatalf("limit message: %s", w.Body)
	}
	expect(t, bob.do("PUT", "/api/workflows/ai-digest/graph", `{"nodes":[]}`), http.StatusForbidden, "edit an example")

	// Once an account owns it, only that account can run it.
	owner := domain.Member{ID: "mem_owner", WorkspaceID: p.WorkspaceID, Name: "Owner", Email: "owner@test.dev", Initials: "O", Role: domain.MemberMember}
	if err := p.Repos.Accounts.Create(owner, "unused"); err != nil {
		t.Fatal(err)
	}
	expect(t, bob.do("POST", "/api/workflows/ai-digest/runs", `{}`), http.StatusForbidden, "run someone else's workflow")

	alice := signedInAs(t, p, h, owner.ID)
	expect(t, alice.do("POST", "/api/workflows/ai-digest/runs", `{}`), http.StatusCreated, "owner runs it")
}

func TestCredentialsArePrivate(t *testing.T) {
	p, h := newTestAPI(t)
	alice := &apiClient{t: t, h: h}
	bob := &apiClient{t: t, h: h}
	expect(t, alice.do("POST", "/api/auth/signup", `{"email":"alice@test.dev","password":"alice password"}`), http.StatusCreated, "alice")
	expect(t, bob.do("POST", "/api/auth/signup", `{"email":"bob@test.dev","password":"bob password"}`), http.StatusCreated, "bob")

	w := alice.do("POST", "/api/credentials", `{"name":"My GitHub","type":"github","values":{"GITHUB_TOKEN":"ghp_x"}}`)
	expect(t, w, http.StatusCreated, "alice saves a credential")
	var cred struct{ ID string }
	_ = json.Unmarshal(w.Body.Bytes(), &cred)

	if body := bob.do("GET", "/api/credentials", "").Body.String(); strings.Contains(body, cred.ID) {
		t.Fatalf("bob sees alice's credential: %s", body)
	}
	expect(t, bob.do("DELETE", "/api/credentials/"+cred.ID, ""), http.StatusNotFound, "bob deletes alice's credential")
	expect(t, bob.do("PUT", "/api/credentials/"+cred.ID, `{"name":"mine now"}`), http.StatusNotFound, "bob renames alice's credential")
	if body := alice.do("GET", "/api/credentials", "").Body.String(); !strings.Contains(body, cred.ID) {
		t.Fatalf("alice should see her credential: %s", body)
	}

	// A graph that names alice's credential cannot run as bob.
	if _, err := p.Workflows.SaveGraph("ai-digest", []domain.GraphNode{{
		Key: "gh", Name: "GitHub", Type: "github.listIssues", Config: map[string]any{"__credentialId": cred.ID},
	}}); err != nil {
		t.Fatal(err)
	}
	w = bob.do("POST", "/api/workflows/ai-digest/runs", `{"skipCredentialCheck":true}`)
	expect(t, w, http.StatusForbidden, "bob runs with alice's credential")
}

// signedInAs starts a session for an existing account directly in the store.
func signedInAs(t *testing.T, p *services.Platform, h http.Handler, id domain.MemberId) *apiClient {
	t.Helper()
	token := "test-token-" + string(id)
	sum := sha256.Sum256([]byte(token))
	if err := p.Repos.Accounts.CreateSession(hex.EncodeToString(sum[:]), id, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return &apiClient{t: t, h: h, cookie: &http.Cookie{Name: sessionCookie, Value: token}}
}
