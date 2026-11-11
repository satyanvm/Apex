package handler

// accounts.go — email + password sign-in.
//
//	POST /api/auth/signup   {name?, email, password} → {user}, sets the session cookie
//	POST /api/auth/signin   {email, password}        → {user}, sets the session cookie
//	POST /api/auth/signout                           → 204, clears it
//	GET  /api/auth/me                                → {user} or {user: null}
//
// Every other non-public /api route needs a signed-in session (sessionMiddleware).
// The landing page, the playground's compile (/api/public/*), health and webhook
// ingress stay open.

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/services"
)

const sessionCookie = "apex_session"

type memberKey struct{}

// accountUser is what the web app sees of the signed-in member.
type accountUser struct {
	ID       domain.MemberId `json:"id"`
	Name     string          `json:"name"`
	Email    string          `json:"email"`
	Initials string          `json:"initials"`
}

func userOf(m domain.Member) accountUser {
	return accountUser{ID: m.ID, Name: m.Name, Email: m.Email, Initials: m.Initials}
}

// sessionMiddleware requires a valid session cookie on every route that is not
// public, and puts the member on the request context.
func sessionMiddleware(p *services.Platform) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodOptions || authExempt(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			m, ok := memberFromCookie(p, r)
			if !ok {
				fail(w, domain.CodeUnauthorized, "Sign in to continue.", nil)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), memberKey{}, m)))
		})
	}
}

func memberFromCookie(p *services.Platform, r *http.Request) (domain.Member, bool) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return domain.Member{}, false
	}
	return p.Accounts.MemberForToken(c.Value)
}

// currentMember is the signed-in member; routes behind sessionMiddleware always
// have one.
func currentMember(r *http.Request) (domain.Member, bool) {
	m, ok := r.Context().Value(memberKey{}).(domain.Member)
	return m, ok
}

// actorID is the signed-in member's id, or nil outside a session.
func actorID(r *http.Request) *domain.MemberId {
	if m, ok := currentMember(r); ok {
		return &m.ID
	}
	return nil
}

func setSessionCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookie, Value: token, Path: "/", MaxAge: maxAge,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: os.Getenv("ENV") == "production",
	})
}

type authHandlers struct {
	p *services.Platform
	// attempts bounds password guessing; signups bounds account creation.
	// Both per client address.
	attempts *services.RateLimiter
	signups  *services.RateLimiter
}

func newAuthHandlers(p *services.Platform) *authHandlers {
	return &authHandlers{
		p:        p,
		attempts: services.NewRateLimiter(10, 15*time.Minute, time.Now),
		signups:  services.NewRateLimiter(5, 24*time.Hour, time.Now),
	}
}

func rateLimited(what string, wait time.Duration) error {
	return domain.Errorf(domain.CodeRateLimited, "Too many %s. Try again in %s.", what, services.WaitText(wait))
}

func (a *authHandlers) signUp(w http.ResponseWriter, r *http.Request) {
	handle(w, func() (any, int, error) {
		if ok, wait := a.signups.Take(clientKey(r)); !ok {
			return nil, 0, rateLimited("new accounts from your network", wait)
		}
		body, _ := decodeBody(r)
		m, token, err := a.p.Accounts.SignUp(getStringField(body, "name"), getStringField(body, "email"), getStringField(body, "password"))
		if err != nil {
			a.signups.Refund(clientKey(r))
			return nil, 0, err
		}
		setSessionCookie(w, token, int(services.SessionTTL.Seconds()))
		return map[string]any{"user": userOf(m)}, http.StatusCreated, nil
	})
}

func (a *authHandlers) signIn(w http.ResponseWriter, r *http.Request) {
	handle(w, func() (any, int, error) {
		if ok, wait := a.attempts.Take(clientKey(r)); !ok {
			return nil, 0, rateLimited("sign-in attempts", wait)
		}
		body, _ := decodeBody(r)
		m, token, err := a.p.Accounts.SignIn(getStringField(body, "email"), getStringField(body, "password"))
		if err != nil {
			return nil, 0, err
		}
		setSessionCookie(w, token, int(services.SessionTTL.Seconds()))
		return map[string]any{"user": userOf(m)}, http.StatusOK, nil
	})
}

func (a *authHandlers) signOut(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.p.Accounts.SignOut(c.Value)
	}
	setSessionCookie(w, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (a *authHandlers) me(w http.ResponseWriter, r *http.Request) {
	handle(w, func() (any, int, error) {
		if m, ok := memberFromCookie(a.p, r); ok {
			return map[string]any{"user": userOf(m)}, http.StatusOK, nil
		}
		return map[string]any{"user": nil}, http.StatusOK, nil
	})
}

/* ------------------------------ ownership ------------------------------ */

// requireEditable allows only the workflow's owner to change it.
func requireEditable(p *services.Platform, r *http.Request, slug string) (domain.Workflow, error) {
	wf, ok := p.Repos.Workflows.GetBySlug(slug)
	if !ok {
		return domain.Workflow{}, domain.NotFound("workflow")
	}
	m, _ := currentMember(r)
	if wf.OwnerID == nil || *wf.OwnerID != m.ID {
		return domain.Workflow{}, domain.Forbidden("Only the account that created this workflow can change it.")
	}
	return wf, nil
}

// requireRunnable allows the owner, and anyone for a workflow no account owns
// (the examples that shipped with the site). Runs always use the runner's own
// credentials, so running someone else's example spends nothing of theirs.
func requireRunnable(p *services.Platform, r *http.Request, slug string) (domain.Workflow, error) {
	wf, ok := p.Repos.Workflows.GetBySlug(slug)
	if !ok {
		return domain.Workflow{}, domain.NotFound("workflow")
	}
	m, _ := currentMember(r)
	if wf.OwnerID != nil && *wf.OwnerID != m.ID && p.Accounts.HasAccount(*wf.OwnerID) {
		return domain.Workflow{}, domain.Forbidden("Only the account that created this workflow can run it.")
	}
	return wf, nil
}

// takeLimit spends one of the member's daily actions on l.
func takeLimit(l *services.RateLimiter, r *http.Request, what string) error {
	m, _ := currentMember(r)
	if ok, wait := l.Take(string(m.ID)); !ok {
		return domain.Errorf(domain.CodeRateLimited, "You've used all %d %s for the last 24 hours. Try again in %s.",
			l.Limit(), what, services.WaitText(wait))
	}
	return nil
}

func refundLimit(l *services.RateLimiter, r *http.Request) {
	m, _ := currentMember(r)
	l.Refund(string(m.ID))
}
