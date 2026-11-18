package services

// google_signin.go — "Continue with Google" for Apex accounts (OpenID Connect,
// authorization-code flow).
//
//	1. GET /api/auth/google/start?next=/x → 302 to Google, with a random state
//	   that is also put in a short-lived HttpOnly cookie.
//	2. Google redirects to /api/auth/google/callback?code=…&state=…; the state
//	   must match the cookie.
//	3. The code is exchanged at Google's token endpoint over TLS, so the id_token
//	   in that response is trusted without checking its signature (OIDC Core
//	   3.1.3.7). Its audience, issuer, expiry and email_verified are still checked.
//	4. The verified email signs in to its account, or creates one.
//
// Uses the same OAuth client as the Gmail integration (GOOGLE_CLIENT_ID /
// GOOGLE_CLIENT_SECRET). Its redirect URI is APP_BASE_URL +
// /api/auth/google/callback, unless GOOGLE_SIGNIN_REDIRECT overrides it.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/agently/api/internal/domain"
)

const googleSignInScopes = "openid email profile"

// GoogleSignIn holds the OAuth client settings for Google sign-in.
type GoogleSignIn struct {
	clientID     string
	clientSecret string
	redirectURL  string
	http         *http.Client
	now          func() time.Time
	// tokenURL is Google's token endpoint; tests point it elsewhere.
	tokenURL string
}

func NewGoogleSignIn(client *http.Client) *GoogleSignIn {
	base := strings.TrimRight(envOrDefault("APP_BASE_URL", "http://localhost:3000"), "/")
	return &GoogleSignIn{
		clientID:     strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
		clientSecret: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET")),
		redirectURL:  envOrDefault("GOOGLE_SIGNIN_REDIRECT", base+"/api/auth/google/callback"),
		http:         client,
		now:          time.Now,
		tokenURL:     "https://oauth2.googleapis.com/token",
	}
}

// Configured reports whether Google sign-in can be offered.
func (g *GoogleSignIn) Configured() bool {
	return g != nil && g.clientID != "" && g.clientSecret != ""
}

// NewState returns a random value for the state parameter and its cookie.
func (g *GoogleSignIn) NewState() string { return randToken() }

// AuthURL is Google's consent page for this sign-in attempt.
func (g *GoogleSignIn) AuthURL(state string) string {
	q := url.Values{
		"client_id":     {g.clientID},
		"redirect_uri":  {g.redirectURL},
		"response_type": {"code"},
		"scope":         {googleSignInScopes},
		"state":         {state},
		"prompt":        {"select_account"},
	}
	return "https://accounts.google.com/o/oauth2/v2/auth?" + q.Encode()
}

// GoogleIdentity is what Apex keeps from a verified Google id_token.
type GoogleIdentity struct {
	Email string
	Name  string
}

// Exchange trades the authorization code for the signed-in Google identity.
func (g *GoogleSignIn) Exchange(ctx context.Context, code string) (GoogleIdentity, error) {
	if code == "" {
		return GoogleIdentity{}, domain.BadRequest("missing authorization code")
	}
	form := url.Values{
		"code":          {code},
		"client_id":     {g.clientID},
		"client_secret": {g.clientSecret},
		"redirect_uri":  {g.redirectURL},
		"grant_type":    {"authorization_code"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return GoogleIdentity{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.http.Do(req)
	if err != nil {
		return GoogleIdentity{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return GoogleIdentity{}, domain.Errorf(domain.CodeInternal, "Google token exchange failed (status %d)", resp.StatusCode)
	}
	var tok struct {
		IDToken string `json:"id_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return GoogleIdentity{}, err
	}
	return g.identity(tok.IDToken)
}

// identity checks the id_token's claims and returns the verified identity.
func (g *GoogleSignIn) identity(idToken string) (GoogleIdentity, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return GoogleIdentity{}, domain.Unauthorized("Google did not return an identity token.")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return GoogleIdentity{}, domain.Unauthorized("Google returned an unreadable identity token.")
	}
	var c struct {
		Iss           string `json:"iss"`
		Aud           string `json:"aud"`
		Exp           int64  `json:"exp"`
		Email         string `json:"email"`
		EmailVerified any    `json:"email_verified"` // bool, or "true" in older tokens
		Name          string `json:"name"`
	}
	if err := json.Unmarshal(payload, &c); err != nil {
		return GoogleIdentity{}, domain.Unauthorized("Google returned an unreadable identity token.")
	}
	switch {
	case c.Iss != "https://accounts.google.com" && c.Iss != "accounts.google.com":
		return GoogleIdentity{}, domain.Unauthorized("The identity token was not issued by Google.")
	case c.Aud != g.clientID:
		return GoogleIdentity{}, domain.Unauthorized("The identity token was issued for another app.")
	case c.Exp != 0 && g.now().Unix() > c.Exp:
		return GoogleIdentity{}, domain.Unauthorized("The identity token has expired. Try again.")
	case c.Email == "":
		return GoogleIdentity{}, domain.Unauthorized("Google did not share an email address.")
	case c.EmailVerified != true && c.EmailVerified != "true":
		return GoogleIdentity{}, domain.Unauthorized("Your Google email address is not verified.")
	}
	return GoogleIdentity{Email: c.Email, Name: c.Name}, nil
}

// SignInWithGoogle signs a verified Google identity in, creating the account on
// first use.
//
// Email sign-ups are not verified, so someone could register a password account
// on an address they don't own before its owner arrives. When Google proves the
// address, that unverified password is dropped and its sessions are ended, so
// only the address's owner keeps access. From then on the account signs in with
// Google.
func (s *AccountService) SignInWithGoogle(id GoogleIdentity) (domain.Member, string, error) {
	email, err := normalizeEmail(id.Email)
	if err != nil {
		return domain.Member{}, "", err
	}
	if m, hash, ok := s.deps.Repos.Accounts.ByEmail(email); ok {
		if hash != "" {
			if err := s.deps.Repos.Accounts.ClaimVerified(m.ID); err != nil {
				return domain.Member{}, "", domain.Errorf(domain.CodeInternal, "could not sign in")
			}
		}
		token, err := s.startSession(m.ID)
		return m, token, err
	}

	name := strings.TrimSpace(id.Name)
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	m := domain.Member{
		ID: domain.NewMemberId(), WorkspaceID: s.deps.Repos.Workspace.Get().ID,
		Name: name, Email: email, Initials: initialsOf(name), Role: domain.MemberMember,
		CreatedAt: domain.Timestamp(s.deps.Clock.ISO()),
	}
	// An empty password hash never matches, so the account signs in only with Google.
	if err := s.deps.Repos.Accounts.Create(m, ""); err != nil {
		return domain.Member{}, "", domain.Errorf(domain.CodeInternal, "could not create the account")
	}
	token, err := s.startSession(m.ID)
	return m, token, err
}
