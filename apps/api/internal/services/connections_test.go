package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agently/api/internal/domain"
)

/* ---------------------------- Google sign-in --------------------------- */

func idToken(claims map[string]any) string {
	b, _ := json.Marshal(claims)
	return "e30." + base64.RawURLEncoding.EncodeToString(b) + ".sig"
}

func TestGoogleIdentityChecksClaims(t *testing.T) {
	g := &GoogleSignIn{clientID: "client-1", now: func() time.Time { return time.Unix(1000, 0) }}
	good := map[string]any{"iss": "https://accounts.google.com", "aud": "client-1", "exp": 2000, "email": "a@b.co", "email_verified": true, "name": "Ada L"}
	id, err := g.identity(idToken(good))
	if err != nil || id.Email != "a@b.co" || id.Name != "Ada L" {
		t.Fatalf("identity = %+v, %v", id, err)
	}
	for name, mut := range map[string]func(map[string]any){
		"wrong audience": func(c map[string]any) { c["aud"] = "other" },
		"wrong issuer":   func(c map[string]any) { c["iss"] = "https://evil.example" },
		"expired":        func(c map[string]any) { c["exp"] = 999 },
		"unverified":     func(c map[string]any) { c["email_verified"] = false },
		"no email":       func(c map[string]any) { delete(c, "email") },
	} {
		c := map[string]any{}
		for k, v := range good {
			c[k] = v
		}
		mut(c)
		if _, err := g.identity(idToken(c)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSignInWithGoogleCreatesThenReusesAccount(t *testing.T) {
	p := newTestPlatform()
	m1, tok1, err := p.Accounts.SignInWithGoogle(GoogleIdentity{Email: "New@Example.com", Name: "New Person"})
	if err != nil || tok1 == "" || m1.Email != "new@example.com" || m1.Initials != "NP" {
		t.Fatalf("first sign-in = %+v, %q, %v", m1, tok1, err)
	}
	m2, _, err := p.Accounts.SignInWithGoogle(GoogleIdentity{Email: "new@example.com"})
	if err != nil || m2.ID != m1.ID {
		t.Fatalf("second sign-in made another account: %v vs %v (%v)", m2.ID, m1.ID, err)
	}
	if _, _, err := p.Accounts.SignIn("new@example.com", ""); err == nil || !strings.Contains(err.Error(), "Google") {
		t.Fatalf("password sign-in to a Google account: %v", err)
	}
}

func TestSignInWithGoogleClaimsAnUnverifiedPasswordAccount(t *testing.T) {
	p := newTestPlatform()
	squatter, squatterToken, err := p.Accounts.SignUp("", "owner@example.com", "squatter-pass")
	if err != nil {
		t.Fatal(err)
	}
	m, _, err := p.Accounts.SignInWithGoogle(GoogleIdentity{Email: "owner@example.com"})
	if err != nil || m.ID != squatter.ID {
		t.Fatalf("google sign-in = %v, %v", m.ID, err)
	}
	if _, ok := p.Accounts.MemberForToken(squatterToken); ok {
		t.Fatal("the earlier password session survived")
	}
	if _, _, err := p.Accounts.SignIn("owner@example.com", "squatter-pass"); err == nil {
		t.Fatal("the unverified password still works")
	}
}

/* ------------------------------- OAuth2 -------------------------------- */

func TestProviderOfAndClient(t *testing.T) {
	cases := map[string]string{
		"https://accounts.google.com/o/oauth2/auth":                      "GOOGLE",
		"https://slack.com/oauth/v2/authorize?user_scope=x":              "SLACK",
		"https://github.com/login/oauth/authorize":                       "GITHUB",
		"https://login.microsoftonline.com/common/oauth2/v2.0/authorize": "MICROSOFT",
		"https://app.example.co.uk/oauth":                                "EXAMPLE",
		"not a url":                                                      "",
	}
	for in, want := range cases {
		if got := providerOf(in); got != want {
			t.Errorf("providerOf(%q) = %q, want %q", in, got, want)
		}
	}

	t.Setenv("GOOGLE_CLIENT_ID", "g-id")
	t.Setenv("GOOGLE_CLIENT_SECRET", "g-secret")
	if id, secret, ok := oauthClient("pieces.gmail", "https://accounts.google.com/o/oauth2/auth"); !ok || id != "g-id" || secret != "g-secret" {
		t.Fatalf("google fallback = %q %q %v", id, secret, ok)
	}
	t.Setenv("OAUTH_GMAIL_CLIENT_ID", "gm-id")
	t.Setenv("OAUTH_GMAIL_CLIENT_SECRET", "gm-secret")
	if id, _, _ := oauthClient("pieces.gmail", "https://accounts.google.com/o/oauth2/auth"); id != "gm-id" {
		t.Fatalf("per-piece client not preferred: %q", id)
	}
	if _, _, ok := oauthClient("pieces.github", "https://github.com/login/oauth/authorize"); ok {
		t.Fatal("github has no client configured")
	}
}

func TestUsableApp(t *testing.T) {
	ok := OAuthApp{AuthURL: "https://a.example/auth", TokenURL: "https://a.example/token"}
	if !usableApp(ok) {
		t.Fatal("plain https app rejected")
	}
	for _, bad := range []OAuthApp{
		{AuthURL: "https://{cloud}/auth", TokenURL: "https://a.example/token"},
		{AuthURL: "http://a.example/auth", TokenURL: "https://a.example/token"},
		{AuthURL: "https://a.example/auth", TokenURL: ""},
	} {
		if usableApp(bad) {
			t.Errorf("accepted %+v", bad)
		}
	}
}

func TestParseTokenResponse(t *testing.T) {
	if tok := parseTokenResponse([]byte(`{"access_token":"a","expires_in":3600}`)); tok["access_token"] != "a" {
		t.Fatalf("json: %v", tok)
	}
	if tok := parseTokenResponse([]byte("access_token=b&scope=repo&token_type=bearer")); tok["access_token"] != "b" || tok["scope"] != "repo" {
		t.Fatalf("form: %v", tok)
	}
}

func TestOAuthConnectStartAndFinish(t *testing.T) {
	var mu sync.Mutex
	var form url.Values
	token := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		form, _ = url.ParseQuery(string(body))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ya29.x","refresh_token":"1//r","expires_in":3599,"token_type":"Bearer","scope":"s"}`))
	}))
	defer token.Close()

	t.Setenv("OAUTH_GMAIL_CLIENT_ID", "cid")
	t.Setenv("OAUTH_GMAIL_CLIENT_SECRET", "csecret")
	p := newTestPlatform()
	svc := NewOAuthConnectService(p.Credentials, token.Client())
	svc.redirectURL = "https://apex.test/api/oauth/callback"
	svc.fetchApp = func(context.Context) (map[string]OAuthApp, error) {
		return map[string]OAuthApp{"pieces.gmail": {
			AuthURL: "https://accounts.google.com/o/oauth2/auth", TokenURL: token.URL,
			Scope: []string{"https://www.googleapis.com/auth/gmail.send", "email"}, PKCE: true,
		}}, nil
	}

	if got := svc.Available(context.Background()); len(got) != 1 || got[0] != "pieces.gmail" {
		t.Fatalf("Available = %v", got)
	}
	owner, other := domain.MemberId("mem_owner"), domain.MemberId("mem_other")
	authURL, err := svc.Start(context.Background(), owner, "pieces.gmail", "")
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	q := u.Query()
	if q.Get("client_id") != "cid" || q.Get("access_type") != "offline" || q.Get("code_challenge_method") != "S256" ||
		q.Get("scope") != "https://www.googleapis.com/auth/gmail.send email" || q.Get("redirect_uri") != svc.redirectURL {
		t.Fatalf("auth url query = %v", q)
	}
	state := q.Get("state")

	if _, err := svc.Finish(context.Background(), other, state, "code-1", ""); err == nil {
		t.Fatal("another member finished someone else's connection")
	}
	cred, err := svc.Finish(context.Background(), owner, state, "code-1", "")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	if form.Get("code") != "code-1" || form.Get("client_secret") != "csecret" || form.Get("code_verifier") == "" {
		t.Fatalf("token request = %v", form)
	}
	mu.Unlock()
	if cred.Type != "pieces.gmail" || cred.Name != "Gmail account" {
		t.Fatalf("credential = %+v", cred)
	}
	_, data, err := p.Credentials.Data(&owner, cred.ID)
	if err != nil || data["access_token"] != "ya29.x" || data["refresh_token"] != "1//r" || data["token_url"] != token.URL || data["client_id"] != "cid" {
		t.Fatalf("stored data = %v (%v)", data, err)
	}
	if _, err := svc.Finish(context.Background(), owner, state, "code-1", ""); err == nil {
		t.Fatal("a state was usable twice")
	}
}

func TestTypedCredentialsCannotUsePlatformValues(t *testing.T) {
	p := newTestPlatform()
	owner := domain.MemberId("mem_x")
	if _, err := p.Credentials.Create(&owner, "x", "pieces.telegram-bot", map[string]any{"value": "$platform:telegram"}); err == nil {
		t.Fatal("typed credential took the platform marker")
	}
	c, err := p.Credentials.CreateConnected(&owner, "x", "pieces.telegram-bot", map[string]any{"value": "$platform:telegram", "chat_id": "42"})
	if err != nil {
		t.Fatal(err)
	}
	name := "y"
	if _, err := p.Credentials.Update(&owner, c.ID, &name, map[string]any{"value": " $platform:telegram"}); err == nil {
		t.Fatal("update took the platform marker")
	}
}

/* ------------------------------ Telegram ------------------------------- */

// fakeTelegram is a Bot API that delivers queued messages through getUpdates.
type fakeTelegram struct {
	mu      sync.Mutex
	queue   []map[string]any
	sent    []url.Values
	nextID  int64
	webhook bool
}

func (f *fakeTelegram) push(chatID int64, text string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.queue = append(f.queue, map[string]any{
		"update_id": f.nextID,
		"message":   map[string]any{"text": text, "chat": map[string]any{"id": chatID, "type": "private", "username": "ada"}},
	})
}

func (f *fakeTelegram) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(body))
	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.HasSuffix(r.URL.Path, "/getMe"):
		if strings.Contains(r.URL.Path, "/bot000000:") {
			_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{"username":"apex_test_bot"}}`))
	case strings.HasSuffix(r.URL.Path, "/getUpdates"):
		if f.webhook {
			_, _ = w.Write([]byte(`{"ok":false,"error_code":409,"description":"Conflict: can't use getUpdates method while webhook is active"}`))
			return
		}
		time.Sleep(20 * time.Millisecond)
		f.mu.Lock()
		out := f.queue
		f.queue = nil
		f.mu.Unlock()
		b, _ := json.Marshal(map[string]any{"ok": true, "result": out})
		_, _ = w.Write(b)
	case strings.HasSuffix(r.URL.Path, "/sendMessage"):
		f.mu.Lock()
		f.sent = append(f.sent, form)
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	default:
		http.NotFound(w, r)
	}
}

func waitStatus(t *testing.T, s *TelegramConnectService, owner domain.MemberId, code, want string) TelegramStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		st := s.Status(owner, code)
		if st.State == want {
			return st
		}
		if time.Now().After(deadline) {
			t.Fatalf("status = %+v, want %s", st, want)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTelegramConnectWithPlatformBot(t *testing.T) {
	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	p := newTestPlatform()
	s := NewTelegramConnectService(p.Credentials, p.Logger, srv.Client())
	s.apiBase = srv.URL
	s.platformToken = "123456:" + strings.Repeat("A", 35)

	if info := s.Info(context.Background()); !info.PlatformBot || info.BotUsername != "apex_test_bot" {
		t.Fatalf("info = %+v", info)
	}
	owner := domain.MemberId("mem_tg")
	start, err := s.Start(context.Background(), owner, "pieces.telegram-bot", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if start.Link != "https://t.me/apex_test_bot?start="+start.Code {
		t.Fatalf("link = %s", start.Link)
	}
	if st := s.Status("mem_someone_else", start.Code); st.State != "expired" {
		t.Fatalf("another member sees the connection: %+v", st)
	}
	fake.push(555, "/start wrong-code")
	fake.push(777, "/start "+start.Code)
	st := waitStatus(t, s, owner, start.Code, "connected")
	if st.ChatName != "@ada" {
		t.Fatalf("status = %+v", st)
	}
	_, data, err := p.Credentials.Data(&owner, st.CredentialID)
	if err != nil || data["value"] != TelegramPlatformValue || data["TELEGRAM_BOT_TOKEN"] != TelegramPlatformValue || data["chat_id"] != "777" {
		t.Fatalf("stored = %v (%v)", data, err)
	}
	chats := s.Chats(owner)
	if len(chats) != 1 || chats[0].ChatID != "777" {
		t.Fatalf("chats = %+v", chats)
	}
	fake.mu.Lock()
	replies := len(fake.sent)
	fake.mu.Unlock()
	if replies != 2 { // "expired link" to 555, "connected" to 777
		t.Fatalf("replies = %d", replies)
	}
}

func TestTelegramConnectWithOwnBot(t *testing.T) {
	fake := &fakeTelegram{}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	p := newTestPlatform()
	s := NewTelegramConnectService(p.Credentials, p.Logger, srv.Client())
	s.apiBase = srv.URL
	s.platformToken = ""
	owner := domain.MemberId("mem_own")

	if _, err := s.Start(context.Background(), owner, "telegram", "", ""); err == nil {
		t.Fatal("started without any bot")
	}
	if _, err := s.Start(context.Background(), owner, "telegram", "", "not a token"); err == nil {
		t.Fatal("accepted a malformed token")
	}
	if _, err := s.Start(context.Background(), owner, "telegram", "", "000000:"+strings.Repeat("B", 35)); err == nil {
		t.Fatal("accepted a token Telegram rejects")
	}
	own := "987654:" + strings.Repeat("C", 35)
	start, err := s.Start(context.Background(), owner, "telegram", "Alerts", own)
	if err != nil {
		t.Fatal(err)
	}
	fake.push(42, "/start "+start.Code)
	st := waitStatus(t, s, owner, start.Code, "connected")
	_, data, _ := p.Credentials.Data(&owner, st.CredentialID)
	if data["TELEGRAM_BOT_TOKEN"] != own || data["chat_id"] != "42" {
		t.Fatalf("stored = %v", data)
	}
	if list := p.Credentials.List(&owner); len(list) != 1 || list[0].Name != "Alerts" {
		t.Fatalf("list = %+v", list)
	}
}

func TestTelegramConnectReportsAWebhookBot(t *testing.T) {
	fake := &fakeTelegram{webhook: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()
	p := newTestPlatform()
	s := NewTelegramConnectService(p.Credentials, p.Logger, srv.Client())
	s.apiBase = srv.URL
	owner := domain.MemberId("mem_wh")
	start, err := s.Start(context.Background(), owner, "telegram", "", "987654:"+strings.Repeat("D", 35))
	if err != nil {
		t.Fatal(err)
	}
	if st := waitStatus(t, s, owner, start.Code, "failed"); !strings.Contains(st.Error, "webhook") {
		t.Fatalf("status = %+v", st)
	}
}
