package handler

// connections.go — Google sign-in and one-click account connections.
//
//	GET  /api/auth/providers               → {google: bool}
//	GET  /api/auth/google/start?next=/x    → 302 to Google (browser navigation)
//	GET  /api/auth/google/callback         → session cookie, 302 to next
//
//	GET  /api/oauth/apps                   → credential types with one-click connect
//	GET  /api/oauth/{type}/start?name=…    → 302 to the provider (in a popup)
//	GET  /api/oauth/callback               → saves the credential, tells the opener
//
//	GET  /api/connect/telegram             → whether the server has its own bot
//	POST /api/connect/telegram/start       → {credentialType, name?, botToken?} → deep link + code
//	GET  /api/connect/telegram/status?code → waiting | connected | expired | failed
//	GET  /api/connect/telegram/chats       → the member's connected chats

import (
	"encoding/json"
	"html"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/services"
	"github.com/go-chi/chi/v5"
)

const googleStateCookie = "apex_google_state"

func authProviders(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{"google": p.GoogleSignIn.Configured()}, http.StatusOK)
	}
}

// safeNextPath keeps a post-sign-in redirect on this site.
func safeNextPath(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.HasPrefix(next, "/\\") {
		return next
	}
	return "/dashboard"
}

func (a *authHandlers) googleStart(w http.ResponseWriter, r *http.Request) {
	if !a.p.GoogleSignIn.Configured() {
		http.Redirect(w, r, "/signin?error=google", http.StatusFound)
		return
	}
	state := a.p.GoogleSignIn.NewState()
	next := safeNextPath(r.URL.Query().Get("next"))
	http.SetCookie(w, &http.Cookie{
		Name: googleStateCookie, Value: state + "|" + url.QueryEscape(next),
		Path: "/api/auth/google", MaxAge: 600, HttpOnly: true,
		SameSite: http.SameSiteLaxMode, Secure: os.Getenv("ENV") == "production",
	})
	http.Redirect(w, r, a.p.GoogleSignIn.AuthURL(state), http.StatusFound)
}

func (a *authHandlers) googleCallback(w http.ResponseWriter, r *http.Request) {
	fail := func(reason string) {
		a.p.Logger.Warn("google sign-in failed", "reason", reason)
		http.Redirect(w, r, "/signin?error=google", http.StatusFound)
	}
	c, err := r.Cookie(googleStateCookie)
	http.SetCookie(w, &http.Cookie{Name: googleStateCookie, Path: "/api/auth/google", MaxAge: -1})
	if err != nil {
		fail("missing state cookie")
		return
	}
	state, rawNext, _ := strings.Cut(c.Value, "|")
	q := r.URL.Query()
	if state == "" || q.Get("state") != state {
		fail("state mismatch")
		return
	}
	if e := q.Get("error"); e != "" {
		fail("google: " + e)
		return
	}
	if ok, _ := a.attempts.Take(clientKey(r)); !ok {
		fail("rate limited")
		return
	}
	id, err := a.p.GoogleSignIn.Exchange(r.Context(), q.Get("code"))
	if err != nil {
		fail(err.Error())
		return
	}
	_, token, err := a.p.Accounts.SignInWithGoogle(id)
	if err != nil {
		fail(err.Error())
		return
	}
	setSessionCookie(w, token, int(services.SessionTTL.Seconds()))
	next, _ := url.QueryUnescape(rawNext)
	http.Redirect(w, r, safeNextPath(next), http.StatusFound)
}

/* ------------------------------- OAuth2 -------------------------------- */

func oauthApps(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]any{
			"types":       p.OAuthConnect.Available(r.Context()),
			"redirectUrl": p.OAuthConnect.RedirectURL(),
		}, http.StatusOK)
	}
}

func oauthStart(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, _ := currentMember(r)
		target, err := p.OAuthConnect.Start(r.Context(), m.ID, chi.URLParam(r, "type"), r.URL.Query().Get("name"))
		if err != nil {
			connectResultPage(w, map[string]any{"ok": false, "error": errorText(err)})
			return
		}
		http.Redirect(w, r, target, http.StatusFound)
	}
}

func oauthCallback(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, _ := currentMember(r)
		q := r.URL.Query()
		providerErr := q.Get("error")
		if d := q.Get("error_description"); providerErr != "" && d != "" {
			providerErr = d
		}
		cred, err := p.OAuthConnect.Finish(r.Context(), m.ID, q.Get("state"), q.Get("code"), providerErr)
		if err != nil {
			p.Logger.Warn("oauth connect failed", "error", err.Error())
			connectResultPage(w, map[string]any{"ok": false, "error": errorText(err)})
			return
		}
		connectResultPage(w, map[string]any{"ok": true, "credential": cred})
	}
}

// connectResultPage ends the popup: it posts the result to the window that
// opened it (same origin only) and closes. Opened without a popup, it shows the
// result and a link back.
func connectResultPage(w http.ResponseWriter, result map[string]any) {
	result["type"] = "apex-connect"
	payload, _ := json.Marshal(result)
	// Inside <script>, "</" must not appear literally.
	js := strings.ReplaceAll(string(payload), "</", `<\/`)
	msg := "Connected. You can close this window."
	if ok, _ := result["ok"].(bool); !ok {
		msg, _ = result["error"].(string)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Apex</title>
<style>body{font:15px/1.5 system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;padding:16px;background:#0c0d12;color:#e7e8ee;text-align:center}a{color:#8b7cff}</style></head>
<body><div><p>` + html.EscapeString(msg) + `</p><p><a href="/dashboard">Back to Apex</a></p></div>
<script>(function(){var r=` + js + `;try{if(window.opener&&!window.opener.closed){window.opener.postMessage(r,window.location.origin);setTimeout(function(){window.close()},r.ok?150:2500);}}catch(e){}})();</script>
</body></html>`))
}

func errorText(err error) string {
	if de, ok := err.(*domain.DomainError); ok {
		return de.Message
	}
	return "Something went wrong. Try again."
}

/* ------------------------------ Telegram ------------------------------- */

func telegramInfo(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, p.Telegram.Info(r.Context()), http.StatusOK)
	}
}

func telegramStart(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		handle(w, func() (any, int, error) {
			m, _ := currentMember(r)
			body, _ := decodeBody(r)
			out, err := p.Telegram.Start(r.Context(), m.ID,
				getStringField(body, "credentialType"), getStringField(body, "name"), getStringField(body, "botToken"))
			if err != nil {
				return nil, 0, err
			}
			return out, http.StatusOK, nil
		})
	}
}

func telegramStatus(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, _ := currentMember(r)
		writeJSON(w, p.Telegram.Status(m.ID, r.URL.Query().Get("code")), http.StatusOK)
	}
}

func telegramChats(p *services.Platform) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, _ := currentMember(r)
		writeJSON(w, map[string]any{"items": p.Telegram.Chats(m.ID)}, http.StatusOK)
	}
}
