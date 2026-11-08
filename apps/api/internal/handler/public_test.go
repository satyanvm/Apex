package handler

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/services"
)

func testCompiler(compile func(context.Context, string) (services.Plan, error)) *publicCompiler {
	c := newPublicCompiler()
	c.compile = compile
	c.describe = func(string) (string, string, []string) { return "", "", nil }
	return c
}

func okPlan(ctx context.Context, _ string) (services.Plan, error) {
	services.CompileEventForTest(ctx, services.CompileEvent{Stage: "route", Status: "done"})
	return services.Plan{Name: "Demo", Nodes: []domain.GraphNode{{Key: "t", Type: "trigger.manual"}}}, nil
}

func post(h http.Handler, prompt, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"prompt":`+quote(prompt)+`}`))
	req.RemoteAddr = ip + ":4242"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func quote(s string) string { return `"` + s + `"` }

func events(body string) []string {
	var names []string
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		if name, ok := strings.CutPrefix(sc.Text(), "event: "); ok {
			names = append(names, name)
		}
	}
	return names
}

func TestPublicCompileStreamsStagesThenResult(t *testing.T) {
	rec := post(testCompiler(okPlan).handler(), "email me news", "1.1.1.1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if got := strings.Join(events(rec.Body.String()), ","); got != "stage,result" {
		t.Fatalf("events = %q", got)
	}
}

func TestPublicCompileRejectsBadPrompts(t *testing.T) {
	c := testCompiler(okPlan)
	c.maxPrompt = 10
	for name, prompt := range map[string]string{"empty": "   ", "long": strings.Repeat("x", 11)} {
		if rec := post(c.handler(), prompt, "1.1.1.1"); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d", name, rec.Code)
		}
	}
}

func TestPublicCompileRateLimitsPerClient(t *testing.T) {
	var calls atomic.Int32
	c := testCompiler(func(ctx context.Context, p string) (services.Plan, error) {
		calls.Add(1)
		return okPlan(ctx, p)
	})
	c.limits.perClient = 2
	h := c.handler()
	post(h, "one", "1.1.1.1")
	post(h, "two", "1.1.1.1")
	rec := post(h, "three", "1.1.1.1")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("status %d, Retry-After %q", rec.Code, rec.Header().Get("Retry-After"))
	}
	if rec := post(h, "four", "2.2.2.2"); rec.Code != http.StatusOK {
		t.Fatalf("another client was limited: %d", rec.Code)
	}
	if calls.Load() != 3 {
		t.Fatalf("compiled %d times", calls.Load())
	}
}

func TestPublicCompileCachedPromptSpendsNothing(t *testing.T) {
	var calls atomic.Int32
	c := testCompiler(func(ctx context.Context, p string) (services.Plan, error) {
		calls.Add(1)
		return okPlan(ctx, p)
	})
	c.limits.perClient = 1
	h := c.handler()
	post(h, "Email Me News", "1.1.1.1")
	rec := post(h, "email   me news", "1.1.1.1")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cached":true`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if calls.Load() != 1 {
		t.Fatalf("compiled %d times", calls.Load())
	}
}

func TestPublicCompileDailyBudget(t *testing.T) {
	c := testCompiler(okPlan)
	c.limits.dailyBudget = 1
	h := c.handler()
	post(h, "one", "1.1.1.1")
	rec := post(h, "two", "2.2.2.2")
	if rec.Code != http.StatusTooManyRequests || !strings.Contains(rec.Body.String(), "Demo limit reached") {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
}

func TestPublicLimitsResetEachDayAndWindow(t *testing.T) {
	l := newPublicLimits()
	l.perClient, l.dailyBudget = 1, 2
	clock := time.Date(2026, 11, 8, 23, 0, 0, 0, time.UTC)
	l.now = func() time.Time { return clock }

	acquire := func(client string) limitVerdict {
		v, _ := l.acquire(client)
		if v == limitAllowed {
			l.release()
		}
		return v
	}
	if v := acquire("a"); v != limitAllowed {
		t.Fatalf("first: %v", v)
	}
	if v := acquire("a"); v != limitClient {
		t.Fatalf("second: %v", v)
	}
	clock = clock.Add(time.Hour + time.Second)
	if v := acquire("a"); v != limitAllowed {
		t.Fatalf("after window, next day: %v", v)
	}
}

func TestPublicCompileConcurrencyCap(t *testing.T) {
	l := newPublicLimits()
	l.maxInFly = 1
	if v, _ := l.acquire("a"); v != limitAllowed {
		t.Fatalf("first: %v", v)
	}
	if v, _ := l.acquire("b"); v != limitBusy {
		t.Fatalf("second: %v", v)
	}
	l.release()
	if v, _ := l.acquire("b"); v != limitAllowed {
		t.Fatalf("after release: %v", v)
	}
}

func TestPublicCompileHidesProviderErrors(t *testing.T) {
	secret := "upstream said key=AIzaSECRET is over quota"
	c := testCompiler(func(context.Context, string) (services.Plan, error) { return services.Plan{}, errors.New(secret) })
	rec := post(c.handler(), "one", "1.1.1.1")
	if strings.Contains(rec.Body.String(), "AIzaSECRET") || !strings.Contains(rec.Body.String(), "event: error") {
		t.Fatalf("body: %s", rec.Body)
	}
}

func TestClientKeyIgnoresForwardingHeaderUnlessConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "9.9.9.9:1"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	req.Header.Set("X-Apex-Client-IP", "5.6.7.8")
	if got := clientKey(req); got != "9.9.9.9" {
		t.Fatalf("spoofable header was trusted: %s", got)
	}
	t.Setenv("PUBLIC_CLIENT_IP_HEADER", "X-Apex-Client-IP")
	if got := clientKey(req); got != "5.6.7.8" {
		t.Fatalf("configured header ignored: %s", got)
	}
}

func TestClientKeyGroupsIPv6ByPrefix(t *testing.T) {
	a := httptest.NewRequest(http.MethodPost, "/", nil)
	a.RemoteAddr = "[2001:db8:1:2:aaaa::1]:1"
	b := httptest.NewRequest(http.MethodPost, "/", nil)
	b.RemoteAddr = "[2001:db8:1:2:bbbb::2]:1"
	if clientKey(a) != clientKey(b) {
		t.Fatalf("%s != %s", clientKey(a), clientKey(b))
	}
}

func TestAuthExemptsOnlyPublicRoutes(t *testing.T) {
	gate := authMiddleware("token")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	cases := map[string]int{
		"/api/public/compile": http.StatusOK,
		"/api/health":         http.StatusOK,
		"/api/workflows":      http.StatusUnauthorized,
		"/api/credentials":    http.StatusUnauthorized,
		"/api/runs":           http.StatusUnauthorized,
	}
	for path, want := range cases {
		rec := httptest.NewRecorder()
		gate.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != want {
			t.Errorf("%s: got %d, want %d", path, rec.Code, want)
		}
	}
}
