package services

// oauth_connect.go — one-click "Connect" for integrations whose pieces use
// OAuth2 (Gmail, Google Sheets, Slack, GitHub, Notion, …), instead of asking the
// user to paste an access token that expires within the hour.
//
//	1. The web app opens GET /api/oauth/{type}/start in a popup.
//	2. The API reads the piece's authUrl/tokenUrl/scope from the pieces worker
//	   (it loads the piece packages; the API does not) and redirects to the
//	   provider with a random state and, when the piece asks for it, PKCE.
//	3. The provider redirects to /api/oauth/callback. The API checks that the
//	   state was issued to the same signed-in member, exchanges the code, and
//	   saves the tokens as that member's credential.
//	4. The callback page tells the opener the new credential id and closes.
//
// Each provider needs an OAuth app registered by the operator, with redirect
// URI APP_BASE_URL + /api/oauth/callback. Its client id and secret come from
// OAUTH_<PIECE>_CLIENT_ID / _SECRET (e.g. OAUTH_GMAIL_CLIENT_ID), else from
// OAUTH_<PROVIDER>_CLIENT_ID / _SECRET where the provider is named after the
// authorization host (OAUTH_GOOGLE_…, OAUTH_SLACK_…, OAUTH_GITHUB_…,
// OAUTH_MICROSOFT_…). Google also falls back to GOOGLE_CLIENT_ID/_SECRET.
// Types with no configured app keep the manual form.
//
// The saved value has the Activepieces OAuth2 connection shape, including the
// client id/secret and token URL, so the pieces worker can refresh an expired
// access token on its own (apps/pieces-worker/src/oauth-refresh.ts).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/agently/api/internal/domain"
)

// OAuthApp is one piece's OAuth2 settings, as declared by the piece.
type OAuthApp struct {
	AuthURL             string            `json:"authUrl"`
	TokenURL            string            `json:"tokenUrl"`
	Scope               []string          `json:"scope"`
	PKCE                bool              `json:"pkce"`
	PKCEMethod          string            `json:"pkceMethod"`
	Extra               map[string]string `json:"extra"`
	AuthorizationMethod string            `json:"authorizationMethod"` // BODY | HEADER
	Prompt              string            `json:"prompt"`              // none | consent | login | omit
	// Props are the auth props' defaults the URLs were filled from
	// ({cloud} → login.microsoftonline.com); the piece reads them back.
	Props map[string]string `json:"props"`
}

type oauthPending struct {
	owner    domain.MemberId
	credType string
	name     string
	verifier string
	clientID string
	secret   string
	app      OAuthApp
	expires  time.Time
}

// OAuthConnectService runs the connect flow. Pending states live in memory:
// the API is a single process, and a restart only means clicking Connect again.
type OAuthConnectService struct {
	creds       *CredentialService
	http        *http.Client
	workerURL   string
	redirectURL string
	now         func() time.Time

	mu       sync.Mutex
	pending  map[string]oauthPending
	apps     map[string]OAuthApp // credential type → app
	appsAt   time.Time
	appsErr  error
	fetchApp func(ctx context.Context) (map[string]OAuthApp, error)
}

func NewOAuthConnectService(creds *CredentialService, client *http.Client) *OAuthConnectService {
	base := strings.TrimRight(envOrDefault("APP_BASE_URL", "http://localhost:3000"), "/")
	s := &OAuthConnectService{
		creds:       creds,
		http:        client,
		workerURL:   strings.TrimRight(envOrDefault("PIECES_WORKER_URL", "http://localhost:7391"), "/"),
		redirectURL: envOrDefault("OAUTH_REDIRECT_URL", base+"/api/oauth/callback"),
		now:         time.Now,
		pending:     map[string]oauthPending{},
	}
	s.fetchApp = s.fetchWorkerApps
	return s
}

// RedirectURL is the callback URL to register with each provider.
func (s *OAuthConnectService) RedirectURL() string { return s.redirectURL }

// Available lists the credential types that can connect with one click: the
// piece declares OAuth2 and the server has a client for its provider.
func (s *OAuthConnectService) Available(ctx context.Context) []string {
	apps := s.loadApps(ctx)
	out := []string{}
	for typ, app := range apps {
		if _, _, ok := oauthClient(typ, app.AuthURL); ok && usableApp(app) {
			out = append(out, typ)
		}
	}
	sort.Strings(out)
	return out
}

// Start returns the provider URL that begins connecting credType for owner.
func (s *OAuthConnectService) Start(ctx context.Context, owner domain.MemberId, credType, name string) (string, error) {
	app, ok := s.loadApps(ctx)[credType]
	if !ok || !usableApp(app) {
		return "", domain.BadRequest(TypeLabel(credType) + " can't be connected with a sign-in popup. Enter its details instead.")
	}
	clientID, secret, ok := oauthClient(credType, app.AuthURL)
	if !ok {
		return "", domain.BadRequest("Signing in to " + TypeLabel(credType) + " isn't set up on this server yet. Enter its details instead.")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = TypeLabel(credType) + " account"
	}
	if r := []rune(name); len(r) > 120 {
		name = string(r[:120])
	}

	state := randToken()
	p := oauthPending{
		owner: owner, credType: credType, name: name,
		clientID: clientID, secret: secret, app: app,
		expires: s.now().Add(10 * time.Minute),
	}
	q := url.Values{
		"client_id":     {clientID},
		"redirect_uri":  {s.redirectURL},
		"response_type": {"code"},
		"state":         {state},
	}
	if len(app.Scope) > 0 {
		q.Set("scope", strings.Join(app.Scope, " "))
	}
	if app.Prompt != "" && app.Prompt != "omit" {
		q.Set("prompt", app.Prompt)
	}
	if isGoogleAuth(app.AuthURL) {
		// Without these Google returns no refresh token, and the connection
		// stops working after an hour.
		q.Set("access_type", "offline")
		q.Set("prompt", "consent")
	}
	for k, v := range app.Extra {
		q.Set(k, v)
	}
	if app.PKCE {
		p.verifier = randToken() + randToken()
		if strings.EqualFold(app.PKCEMethod, "plain") {
			q.Set("code_challenge", p.verifier)
			q.Set("code_challenge_method", "plain")
		} else {
			sum := sha256.Sum256([]byte(p.verifier))
			q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
			q.Set("code_challenge_method", "S256")
		}
	}

	s.mu.Lock()
	for k, v := range s.pending {
		if s.now().After(v.expires) {
			delete(s.pending, k)
		}
	}
	s.pending[state] = p
	s.mu.Unlock()

	sep := "?"
	if strings.Contains(app.AuthURL, "?") {
		sep = "&"
	}
	return app.AuthURL + sep + q.Encode(), nil
}

// Finish handles the provider's redirect and saves the credential.
func (s *OAuthConnectService) Finish(ctx context.Context, owner domain.MemberId, state, code, providerErr string) (CredentialSummary, error) {
	s.mu.Lock()
	p, ok := s.pending[state]
	if ok && p.owner == owner {
		delete(s.pending, state)
	}
	s.mu.Unlock()
	if !ok || p.owner != owner || s.now().After(p.expires) {
		return CredentialSummary{}, domain.BadRequest("This sign-in link expired. Close this window and click Connect again.")
	}
	if providerErr != "" {
		if providerErr == "access_denied" {
			return CredentialSummary{}, domain.BadRequest("Access was not granted.")
		}
		return CredentialSummary{}, domain.BadRequest(TypeLabel(p.credType) + " returned an error: " + providerErr)
	}
	if code == "" {
		return CredentialSummary{}, domain.BadRequest("The provider did not return an authorization code.")
	}

	tok, err := s.exchange(ctx, p, code)
	if err != nil {
		return CredentialSummary{}, err
	}
	access, _ := tok["access_token"].(string)
	if access == "" {
		return CredentialSummary{}, domain.BadRequest(TypeLabel(p.credType) + " did not return an access token.")
	}
	method := p.app.AuthorizationMethod
	if method == "" {
		method = "BODY"
	}
	values := map[string]any{
		"access_token":         access,
		"token_type":           tok["token_type"],
		"expires_in":           tok["expires_in"],
		"refresh_token":        tok["refresh_token"],
		"scope":                tok["scope"],
		"claimed_at":           s.now().Unix(),
		"data":                 tok,
		"client_id":            p.clientID,
		"client_secret":        p.secret,
		"token_url":            p.app.TokenURL,
		"redirect_url":         s.redirectURL,
		"authorization_method": method,
		"props":                propsOf(p.app),
		"type":                 "OAUTH2",
	}
	return s.creds.CreateConnected(&owner, p.name, p.credType, values)
}

func (s *OAuthConnectService) exchange(ctx context.Context, p oauthPending, code string) (map[string]any, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {s.redirectURL},
	}
	if p.verifier != "" {
		form.Set("code_verifier", p.verifier)
	}
	if !strings.EqualFold(p.app.AuthorizationMethod, "HEADER") {
		form.Set("client_id", p.clientID)
		form.Set("client_secret", p.secret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.app.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if strings.EqualFold(p.app.AuthorizationMethod, "HEADER") {
		req.SetBasicAuth(p.clientID, p.secret)
	}
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, domain.Errorf(domain.CodeInternal, "could not reach %s", TypeLabel(p.credType))
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	tok := parseTokenResponse(body)
	if resp.StatusCode/100 != 2 || tok == nil {
		return nil, domain.Errorf(domain.CodeValidationFailed, "%s rejected the sign-in (status %d)%s",
			TypeLabel(p.credType), resp.StatusCode, tokenErrorSuffix(tok))
	}
	// Slack and a few others answer 200 with {"ok": false, "error": "..."}.
	if ok, present := tok["ok"].(bool); present && !ok {
		return nil, domain.Errorf(domain.CodeValidationFailed, "%s rejected the sign-in%s", TypeLabel(p.credType), tokenErrorSuffix(tok))
	}
	if _, isErr := tok["error"]; isErr && tok["access_token"] == nil {
		return nil, domain.Errorf(domain.CodeValidationFailed, "%s rejected the sign-in%s", TypeLabel(p.credType), tokenErrorSuffix(tok))
	}
	return tok, nil
}

func propsOf(app OAuthApp) map[string]any {
	out := map[string]any{}
	for k, v := range app.Props {
		out[k] = v
	}
	return out
}

// parseTokenResponse accepts JSON or a form-encoded body (GitHub's default).
func parseTokenResponse(body []byte) map[string]any {
	var tok map[string]any
	if json.Unmarshal(bytes.TrimSpace(body), &tok) == nil && tok != nil {
		return tok
	}
	vals, err := url.ParseQuery(string(body))
	if err != nil || len(vals) == 0 {
		return nil
	}
	tok = map[string]any{}
	for k := range vals {
		tok[k] = vals.Get(k)
	}
	return tok
}

func tokenErrorSuffix(tok map[string]any) string {
	if tok == nil {
		return ""
	}
	for _, k := range []string{"error_description", "error"} {
		if v, ok := tok[k].(string); ok && v != "" {
			return ": " + v
		}
	}
	return ""
}

/* ----------------------------- piece apps ----------------------------- */

// loadApps returns the OAuth2 apps the pieces worker reports, cached for an
// hour (they change only on redeploy). A failed fetch is retried after 30s and
// keeps serving the last good list meanwhile.
func (s *OAuthConnectService) loadApps(ctx context.Context) map[string]OAuthApp {
	s.mu.Lock()
	age := s.now().Sub(s.appsAt)
	skip := (s.appsErr == nil && s.apps != nil && age < time.Hour) || (s.appsErr != nil && age < 30*time.Second)
	apps := s.apps
	s.mu.Unlock()
	if skip {
		return apps
	}
	got, err := s.fetchApp(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appsAt = s.now()
	s.appsErr = err
	if err == nil {
		s.apps = got
	}
	return s.apps
}

func (s *OAuthConnectService) fetchWorkerApps(ctx context.Context) (map[string]OAuthApp, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.workerURL+"/oauth-apps", strings.NewReader("{}"))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var out struct {
		OK   bool                `json:"ok"`
		Apps map[string]OAuthApp `json:"apps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.OK {
		return nil, fmt.Errorf("pieces worker could not list OAuth apps")
	}
	apps := map[string]OAuthApp{}
	for slug, app := range out.Apps {
		apps["pieces."+slug] = app
	}
	return apps, nil
}

// usableApp rejects apps whose URLs depend on per-connection input, such as
// "https://{subdomain}.zendesk.com/…": those still use the manual form.
func usableApp(app OAuthApp) bool {
	ok := func(u string) bool {
		parsed, err := url.Parse(u)
		return err == nil && parsed.Scheme == "https" && parsed.Host != "" && !strings.ContainsAny(u, "{}")
	}
	return ok(app.AuthURL) && ok(app.TokenURL)
}

func isGoogleAuth(authURL string) bool {
	u, err := url.Parse(authURL)
	return err == nil && u.Host == "accounts.google.com"
}

var nonEnv = regexp.MustCompile(`[^A-Z0-9]+`)

// providerAliases names providers whose authorization host doesn't say it.
var providerAliases = map[string]string{
	"microsoftonline": "MICROSOFT",
	"live":            "MICROSOFT",
}

// oauthClient finds the server's OAuth client for a credential type.
func oauthClient(credType, authURL string) (string, string, bool) {
	slug := strings.TrimPrefix(credType, "pieces.")
	prefixes := []string{"OAUTH_" + nonEnv.ReplaceAllString(strings.ToUpper(slug), "_")}
	if provider := providerOf(authURL); provider != "" {
		prefixes = append(prefixes, "OAUTH_"+provider)
		if provider == "GOOGLE" {
			prefixes = append(prefixes, "GOOGLE")
		}
	}
	for _, p := range prefixes {
		id := strings.TrimSpace(os.Getenv(p + "_CLIENT_ID"))
		secret := strings.TrimSpace(os.Getenv(p + "_CLIENT_SECRET"))
		if id != "" && secret != "" {
			return id, secret, true
		}
	}
	return "", "", false
}

// providerOf names the provider from its authorization host:
// accounts.google.com → GOOGLE, slack.com → SLACK, github.com → GITHUB.
func providerOf(authURL string) string {
	u, err := url.Parse(authURL)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	parts := strings.Split(u.Hostname(), ".")
	if len(parts) < 2 {
		return ""
	}
	label := parts[len(parts)-2]
	// Two-part public suffixes such as co.uk, com.au.
	if len(parts) >= 3 && len(label) <= 3 && (label == "co" || label == "com" || label == "net" || label == "org") {
		label = parts[len(parts)-3]
	}
	if alias, ok := providerAliases[label]; ok {
		return alias
	}
	return nonEnv.ReplaceAllString(strings.ToUpper(label), "_")
}
