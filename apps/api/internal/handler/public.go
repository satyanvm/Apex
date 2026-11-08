package handler

// public.go serves the anonymous playground. POST /api/public/compile streams a
// real compile stage by stage and returns the graph. It never saves a workflow
// and never starts a run, so the only thing a caller can spend is model tokens,
// and public_limits.go bounds those.

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/agently/api/internal/domain"
	"github.com/agently/api/internal/services"
)

const (
	publicBodyLimit  = 32 << 10
	publicCacheLimit = 128
)

// publicNode is a graph node as the playground shows it.
type publicNode struct {
	Key       string         `json:"key"`
	Name      string         `json:"name"`
	Type      string         `json:"type"`
	Label     string         `json:"label,omitempty"`
	Service   string         `json:"service,omitempty"`
	Kind      string         `json:"kind,omitempty"`
	DependsOn []string       `json:"dependsOn"`
	Config    map[string]any `json:"config"`
	Outputs   []string       `json:"outputs,omitempty"`
	Col       int            `json:"col"`
	Row       int            `json:"row"`
}

type publicResult struct {
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Schedule     *string        `json:"schedule"`
	DefaultInput map[string]any `json:"defaultInput"`
	Sources      []string       `json:"sources"`
	Nodes        []publicNode   `json:"nodes"`
	DurationMs   int64          `json:"durationMs"`
	Repairs      int            `json:"repairs"`
	Cached       bool           `json:"cached"`
}

type publicCompiled struct {
	events []services.CompileEvent
	result publicResult
}

type publicCompiler struct {
	limits    *publicLimits
	compile   func(context.Context, string) (services.Plan, error)
	describe  func(string) nodeInfo
	maxPrompt int
	maxNodes  int
	timeout   time.Duration

	mu    sync.Mutex
	cache map[string]publicCompiled
}

func newPublicCompiler() *publicCompiler {
	return &publicCompiler{
		limits:    newPublicLimits(),
		compile:   services.CompileFresh,
		describe:  describeNodeType,
		maxPrompt: envPositive("PUBLIC_COMPILE_MAX_PROMPT", 1200),
		maxNodes:  envPositive("PUBLIC_COMPILE_MAX_NODES", 40),
		timeout:   time.Duration(envPositive("PUBLIC_COMPILE_TIMEOUT_S", 240)) * time.Second,
		cache:     map[string]publicCompiled{},
	}
}

// nodeInfo is what the catalog says about a node type.
type nodeInfo struct {
	label, service, kind string
	outputs              []string
}

func describeNodeType(nodeType string) nodeInfo {
	label, service, kind := services.CatalogInfo(nodeType)
	return nodeInfo{label: label, service: service, kind: kind, outputs: services.CatalogOutputs(nodeType)}
}

func normalizePrompt(prompt string) string {
	return strings.ToLower(strings.Join(strings.Fields(prompt), " "))
}

func (c *publicCompiler) cached(prompt string) (publicCompiled, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	hit, ok := c.cache[normalizePrompt(prompt)]
	return hit, ok
}

func (c *publicCompiler) store(prompt string, compiled publicCompiled) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= publicCacheLimit {
		c.cache = map[string]publicCompiled{}
	}
	c.cache[normalizePrompt(prompt)] = compiled
}

func (c *publicCompiler) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Prompt string `json:"prompt"`
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, publicBodyLimit))
		if err != nil {
			fail(w, domain.CodeBadRequest, "Request is too large", nil)
			return
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			fail(w, domain.CodeBadRequest, "Body must be JSON with a prompt field", nil)
			return
		}
		prompt := strings.TrimSpace(body.Prompt)
		if prompt == "" {
			fail(w, domain.CodeBadRequest, "Describe the workflow you want", nil)
			return
		}
		if utf8.RuneCountInString(prompt) > c.maxPrompt {
			fail(w, domain.CodeBadRequest, "Keep the prompt under "+strconv.Itoa(c.maxPrompt)+" characters for the public demo", nil)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			fail(w, domain.CodeNotImplemented, "streaming unsupported", nil)
			return
		}

		// A repeated prompt costs nothing, so it does not count against any limit.
		if hit, ok := c.cached(prompt); ok {
			setSSEHeaders(w)
			for _, ev := range hit.events {
				sseFrame(w, flusher, "", "stage", ev)
			}
			result := hit.result
			result.Cached = true
			sseFrame(w, flusher, "", "result", result)
			return
		}

		verdict, retry := c.limits.acquire(clientKey(r))
		if verdict != limitAllowed {
			c.refuse(w, verdict, retry)
			return
		}
		defer c.limits.release()

		setSSEHeaders(w)
		c.stream(w, r, flusher, prompt)
	}
}

func (c *publicCompiler) refuse(w http.ResponseWriter, verdict limitVerdict, retry time.Duration) {
	seconds := int(math.Ceil(retry.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	message := "The demo is busy with other compiles. Try again in a few seconds."
	switch verdict {
	case limitClient:
		message = "You've reached the demo's hourly limit. Try again later, or run Apex yourself from the repo."
	case limitDaily:
		message = "Demo limit reached for today. It resets at midnight UTC."
	}
	fail(w, domain.CodeRateLimited, message, nil)
}

// stream runs one compile, writing events as the planner reports them. Writes
// happen only on this goroutine; the planner's callback hands events over a
// channel so a slow client can't stall the compile, and a disconnect cancels it.
func (c *publicCompiler) stream(w http.ResponseWriter, r *http.Request, flusher http.Flusher, prompt string) {
	ctx, cancel := context.WithTimeout(r.Context(), c.timeout)
	defer cancel()

	type outcome struct {
		plan services.Plan
		err  error
	}
	events := make(chan services.CompileEvent, 64)
	done := make(chan outcome, 1)
	var recorded []services.CompileEvent

	compileCtx := services.WithCompileNodeLimit(services.WithCompileProgress(ctx, func(ev services.CompileEvent) {
		select {
		case events <- ev:
		case <-ctx.Done():
		}
	}), c.maxNodes)

	started := time.Now()
	go func() {
		plan, err := c.compile(compileCtx, prompt)
		done <- outcome{plan, err}
	}()

	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()

	emit := func(ev services.CompileEvent) {
		recorded = append(recorded, ev)
		sseFrame(w, flusher, "", "stage", ev)
	}

	for {
		select {
		case ev := <-events:
			emit(ev)
		case <-heartbeat.C:
			_, _ = io.WriteString(w, ": ping\n\n")
			flusher.Flush()
		case out := <-done:
			for len(events) > 0 {
				emit(<-events)
			}
			if out.err != nil {
				slog.Warn("public compile failed", "error", out.err.Error())
				sseFrame(w, flusher, "", "error", map[string]string{"message": publicCompileError(ctx, out.err)})
				return
			}
			result := c.toResult(out.plan, recorded, time.Since(started))
			c.store(prompt, publicCompiled{events: recorded, result: result})
			sseFrame(w, flusher, "", "result", result)
			return
		}
	}
}

func (c *publicCompiler) toResult(plan services.Plan, events []services.CompileEvent, elapsed time.Duration) publicResult {
	nodes := make([]publicNode, 0, len(plan.Nodes))
	for _, n := range plan.Nodes {
		info := c.describe(n.Type)
		nodes = append(nodes, publicNode{
			Key: n.Key, Name: n.Name, Type: n.Type, Label: info.label, Service: info.service, Kind: info.kind,
			DependsOn: n.DependsOn, Config: n.Config, Outputs: info.outputs, Col: n.Col, Row: n.Row,
		})
	}
	repairs := 0
	for _, ev := range events {
		if ev.Stage == "repair" && ev.Status == "start" {
			repairs++
		}
	}
	return publicResult{
		Name: plan.Name, Description: plan.Description, Schedule: plan.Schedule,
		DefaultInput: plan.DefaultInput, Sources: plan.Sources, Nodes: nodes,
		DurationMs: elapsed.Milliseconds(), Repairs: repairs,
	}
}

// publicCompileError turns a compile failure into a message safe for anonymous
// callers. Provider errors stay in the server log.
func publicCompileError(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, services.ErrGraphTooLarge):
		return "That workflow is too big for the public demo. Try one with fewer steps."
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return "This compile ran out of time. Try a simpler workflow."
	case strings.Contains(err.Error(), "could not author"):
		return "The model couldn't produce a graph that passes validation, even after repairs. Nothing was faked: try rewording the request."
	}
	return "The compile failed on the model provider's side. Try again in a minute."
}
