package services

// compile_progress.go lets a caller watch a compile as it happens. The planner
// reports each stage through a callback carried on the context, so compiles
// without a listener pay nothing and the stage functions keep their signatures.

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// CompileEvent is one step of a compile, in the order it happened.
type CompileEvent struct {
	Stage      string   `json:"stage"`  // route | map | reduce | validate | repair
	Status     string   `json:"status"` // start | done | failed
	Detail     string   `json:"detail,omitempty"`
	Items      []string `json:"items,omitempty"`
	Count      int      `json:"count,omitempty"`
	Total      int      `json:"total,omitempty"`
	Attempt    int      `json:"attempt,omitempty"`
	DurationMs int64    `json:"durationMs,omitempty"`
}

type compileListenerKey struct{}
type compileNodeLimitKey struct{}

// WithCompileProgress returns a context whose compile reports to fn. fn may be
// called from several goroutines' results, but only ever one at a time from the
// planner's sequential stages.
func WithCompileProgress(ctx context.Context, fn func(CompileEvent)) context.Context {
	return context.WithValue(ctx, compileListenerKey{}, fn)
}

// WithCompileNodeLimit makes the compile refuse a graph whose outline estimates
// more than limit nodes, before any node is written. It bounds what one anonymous
// request can spend.
func WithCompileNodeLimit(ctx context.Context, limit int) context.Context {
	return context.WithValue(ctx, compileNodeLimitKey{}, limit)
}

// ErrGraphTooLarge is returned when an outline exceeds the context's node limit.
var ErrGraphTooLarge = errors.New("graph exceeds the node limit for this compile")

func checkCompileNodeLimit(ctx context.Context, estimated int) error {
	limit, _ := ctx.Value(compileNodeLimitKey{}).(int)
	if limit > 0 && estimated > limit {
		return fmt.Errorf("%w: the outline needs about %d nodes, the limit is %d", ErrGraphTooLarge, estimated, limit)
	}
	return nil
}

func emitCompile(ctx context.Context, ev CompileEvent) {
	if fn, ok := ctx.Value(compileListenerKey{}).(func(CompileEvent)); ok && fn != nil {
		fn(ev)
	}
}

// compileStage times one stage and reports its start.
type compileStage struct {
	ctx     context.Context
	stage   string
	attempt int
	started time.Time
}

func startCompileStage(ctx context.Context, stage string, attempt int, detail string) *compileStage {
	emitCompile(ctx, CompileEvent{Stage: stage, Status: "start", Attempt: attempt, Detail: detail})
	return &compileStage{ctx: ctx, stage: stage, attempt: attempt, started: time.Now()}
}

func (s *compileStage) finish(status string, ev CompileEvent) {
	ev.Stage, ev.Status, ev.Attempt = s.stage, status, s.attempt
	ev.DurationMs = time.Since(s.started).Milliseconds()
	emitCompile(s.ctx, ev)
}

func (s *compileStage) done(ev CompileEvent)   { s.finish("done", ev) }
func (s *compileStage) failed(ev CompileEvent) { s.finish("failed", ev) }

// CompileFresh compiles prompt without consulting or filling the plan cache, so
// every stage really runs and reports. Callers that want caching own it.
func CompileFresh(ctx context.Context, prompt string) (Plan, error) {
	if err := RequirePlannerKeys(); err != nil {
		return Plan{}, err
	}
	cat := LoadCatalog()
	p := basePlan(prompt, "")
	nodes, err := compileGraphLLM(ctx, cat, prompt, &p)
	if err != nil {
		return Plan{}, err
	}
	p.Nodes = layoutGraph(nodes)
	if p.Name == "" {
		p.Name = "Untitled Workflow"
	}
	p.Sources = servicesOf(p.Nodes)
	return p, nil
}

// CatalogOutputs returns the declared output fields of a node type, or nil.
func CatalogOutputs(nodeType string) []string {
	if n, ok := LoadCatalog().ByID[nodeType]; ok {
		return n.Outputs
	}
	return nil
}

// CatalogInfo returns the display label, integration name and kind of a node type.
func CatalogInfo(nodeType string) (label, cluster, kind string) {
	cat := LoadCatalog()
	n, ok := cat.ByID[nodeType]
	if !ok {
		return "", "", ""
	}
	return n.Label, cat.Clusters[n.Cluster].Label, n.Kind
}

// CompileEventForTest lets handler tests drive the progress callback.
func CompileEventForTest(ctx context.Context, ev CompileEvent) { emitCompile(ctx, ev) }
