"use client";

import * as React from "react";
import dynamic from "next/dynamic";
import { ArrowRight, Check, Copy, Square } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  applyEvent,
  emptyStages,
  EXAMPLES,
  parseFrame,
  type CompileEvent,
  type CompileResult,
  type Stages,
} from "@/lib/playground";
import { StageTimeline } from "./stage-timeline";
import { NodeInspector } from "./node-inspector";

const GraphView = dynamic(() => import("./graph-view").then((m) => m.GraphView), { ssr: false });

const MAX_PROMPT = 1200;

type Phase = "idle" | "compiling" | "done" | "failed";

export function Playground() {
  const [prompt, setPrompt] = React.useState("");
  const [phase, setPhase] = React.useState<Phase>("idle");
  const [stages, setStages] = React.useState<Stages>(emptyStages);
  const [result, setResult] = React.useState<CompileResult | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [selected, setSelected] = React.useState<string | null>(null);
  const [view, setView] = React.useState<"graph" | "json">("graph");
  const [elapsed, setElapsed] = React.useState(0);
  const abort = React.useRef<AbortController | null>(null);
  const canvas = React.useRef<HTMLElement | null>(null);

  React.useEffect(() => () => abort.current?.abort(), []);

  // On a phone the graph sits below the timeline, out of sight when it lands.
  React.useEffect(() => {
    if (result && window.matchMedia("(max-width: 1023px)").matches) {
      canvas.current?.scrollIntoView({ block: "start" });
    }
  }, [result]);

  React.useEffect(() => {
    if (phase !== "compiling") return;
    const started = Date.now();
    const t = setInterval(() => setElapsed(Date.now() - started), 100);
    return () => clearInterval(t);
  }, [phase]);

  async function compile(text: string) {
    const trimmed = text.trim();
    if (!trimmed || phase === "compiling") return;
    const controller = new AbortController();
    abort.current = controller;
    setPhase("compiling");
    setStages(emptyStages());
    setResult(null);
    setError(null);
    setSelected(null);
    setElapsed(0);

    try {
      const res = await fetch("/api/public/compile", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ prompt: trimmed }),
        signal: controller.signal,
      });
      if (!res.ok || !res.body) {
        const body = await res.json().catch(() => null);
        throw new Error(body?.error?.message ?? "The compiler isn't reachable right now.");
      }
      const reader = res.body.pipeThrough(new TextDecoderStream()).getReader();
      let buffer = "";
      let finished = false;
      for (;;) {
        const { value, done } = await reader.read();
        if (done) break;
        buffer += value;
        let cut: number;
        while ((cut = buffer.indexOf("\n\n")) >= 0) {
          const frame = parseFrame(buffer.slice(0, cut));
          buffer = buffer.slice(cut + 2);
          if (!frame) continue;
          if (frame.event === "stage") {
            setStages((s) => applyEvent(s, frame.data as CompileEvent));
          } else if (frame.event === "result") {
            setResult(frame.data as CompileResult);
            setPhase("done");
            finished = true;
          } else if (frame.event === "error") {
            throw new Error((frame.data as { message: string }).message);
          }
        }
      }
      if (!finished) throw new Error("The connection closed before the compile finished.");
    } catch (e) {
      if (controller.signal.aborted) {
        setPhase("idle");
        setStages(emptyStages());
        return;
      }
      setError(e instanceof Error ? e.message : "Something went wrong.");
      setPhase("failed");
    }
  }

  const compiling = phase === "compiling";
  const selectedNode = result?.nodes.find((n) => n.key === selected) ?? null;

  return (
    <div className="grid gap-px overflow-hidden bg-border lg:h-[calc(100dvh-57px)] lg:grid-cols-[380px_minmax(0,1fr)]">
      <section className="flex flex-col overflow-y-auto bg-bg p-5" aria-label="Compiler">
        <form
          onSubmit={(e) => {
            e.preventDefault();
            void compile(prompt);
          }}
        >
          <label htmlFor="prompt" className="text-[13px] font-medium text-fg">
            Describe an automation
          </label>
          <textarea
            id="prompt"
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) void compile(prompt);
            }}
            maxLength={MAX_PROMPT}
            rows={5}
            placeholder="When a customer cancels in Stripe, ask AI why from their last support tickets, then…"
            className="mt-2 w-full resize-none rounded-md border border-border-strong bg-surface px-3 py-2.5 text-[13.5px] leading-relaxed text-fg outline-none transition-shadow placeholder:text-faint focus:border-accent focus:ring-2 focus:ring-accent/20"
          />
          <div className="mt-2 flex items-center justify-between gap-3">
            <span className="font-mono text-[11px] tabular-nums text-faint">
              {prompt.length}/{MAX_PROMPT}
            </span>
            {compiling ? (
              <button
                type="button"
                onClick={() => abort.current?.abort()}
                className="flex h-9 items-center gap-1.5 rounded-md border border-border-strong px-3.5 text-[13px] font-medium text-fg transition-colors hover:bg-surface-2"
              >
                <Square className="size-3" /> Stop
              </button>
            ) : (
              <button
                type="submit"
                disabled={!prompt.trim()}
                className="flex h-9 items-center gap-1.5 rounded-md bg-accent px-3.5 text-[13px] font-medium text-accent-fg transition-colors hover:bg-accent-soft disabled:cursor-not-allowed disabled:opacity-40"
              >
                Compile <ArrowRight className="size-3.5" />
              </button>
            )}
          </div>
        </form>

        <div className="mt-4">
          <p className="text-[11px] font-medium uppercase tracking-wider text-faint">Or try one</p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {EXAMPLES.map((ex) => (
              <button
                key={ex.label}
                disabled={compiling}
                onClick={() => {
                  setPrompt(ex.prompt);
                  void compile(ex.prompt);
                }}
                className="rounded-full border border-border-strong px-2.5 py-1 text-[12px] text-muted transition-colors hover:border-accent-soft hover:text-fg disabled:opacity-40"
              >
                {ex.label}
              </button>
            ))}
          </div>
        </div>

        <div className="mt-6 border-t border-border pt-5">
          <div className="mb-3 flex items-baseline justify-between">
            <h2 className="text-[13px] font-medium text-fg">Compiler</h2>
            {(compiling || result) && (
              <span className="font-mono text-[11px] tabular-nums text-faint">
                {compiling ? `${(elapsed / 1000).toFixed(1)} s` : result?.cached ? "cached result" : `${((result?.durationMs ?? 0) / 1000).toFixed(1)} s`}
              </span>
            )}
          </div>
          <StageTimeline stages={stages} finished={phase === "done"} />
          {error && (
            <p role="alert" className="mt-4 rounded-md bg-warn-bg px-3 py-2 text-[12.5px] leading-relaxed text-warn">
              {error}
            </p>
          )}
        </div>
      </section>

      <section ref={canvas} className="relative flex h-[75dvh] min-h-[520px] flex-col bg-inset lg:h-auto" aria-label="Compiled workflow">
        {result ? (
          <>
            <header className="flex flex-wrap items-center justify-between gap-x-6 gap-y-2 border-b border-border bg-bg px-5 py-3">
              <div className="min-w-0">
                <h2 className="truncate text-[15px] font-semibold text-fg">{result.name}</h2>
                <p className="mt-0.5 font-mono text-[11px] text-faint">
                  {result.nodes.length} nodes
                  {result.sources?.length ? ` · ${result.sources.length} ${result.sources.length === 1 ? "service" : "services"}` : ""}
                  {` · ${result.repairs} ${result.repairs === 1 ? "repair" : "repairs"}`}
                  {result.schedule ? ` · ${result.schedule}` : ""}
                </p>
              </div>
              <div className="flex rounded-md border border-border-strong p-0.5 text-[12px]" role="tablist">
                {(["graph", "json"] as const).map((v) => (
                  <button
                    key={v}
                    role="tab"
                    aria-selected={view === v}
                    onClick={() => setView(v)}
                    className={cn(
                      "rounded px-2.5 py-1 capitalize transition-colors",
                      view === v ? "bg-surface-3 font-medium text-fg" : "text-muted hover:text-fg",
                    )}
                  >
                    {v === "json" ? "JSON" : "Graph"}
                  </button>
                ))}
              </div>
            </header>
            <div className="relative min-h-0 flex-1">
              {view === "graph" ? (
                <div className="absolute inset-0">
                  <GraphView nodes={result.nodes} selected={selected} onSelect={setSelected} />
                </div>
              ) : (
                <JsonView result={result} />
              )}
              {view === "graph" && selectedNode && (
                <div className="absolute inset-y-0 right-0 z-10 w-full max-w-[340px] border-l border-border bg-bg shadow-pop">
                  <NodeInspector node={selectedNode} nodes={result.nodes} onSelect={setSelected} />
                </div>
              )}
            </div>
          </>
        ) : (
          <EmptyCanvas compiling={compiling} />
        )}
      </section>
    </div>
  );
}

function EmptyCanvas({ compiling }: { compiling: boolean }) {
  return (
    <div className="bg-dots flex flex-1 items-center justify-center p-8 text-center">
      <p className="max-w-[320px] text-[13.5px] leading-relaxed text-muted">
        {compiling
          ? "Choosing from 6,758 real actions. The graph appears once every node and reference has passed validation."
          : "Your workflow appears here as a graph. Select any node to see its configuration and how data reaches it."}
      </p>
    </div>
  );
}

function JsonView({ result }: { result: CompileResult }) {
  const [copied, setCopied] = React.useState(false);
  const text = React.useMemo(
    () =>
      JSON.stringify(
        {
          name: result.name,
          description: result.description,
          schedule: result.schedule,
          defaultInput: result.defaultInput,
          nodes: result.nodes.map(({ key, type, name, dependsOn, config }) => ({ key, type, name, dependsOn, config })),
        },
        null,
        2,
      ),
    [result],
  );
  return (
    <div className="absolute inset-0 overflow-auto bg-bg">
      <button
        onClick={() => {
          void navigator.clipboard.writeText(text).then(() => {
            setCopied(true);
            setTimeout(() => setCopied(false), 1500);
          });
        }}
        className="absolute right-4 top-3 flex items-center gap-1.5 rounded-md border border-border-strong bg-surface px-2.5 py-1 text-[12px] text-muted transition-colors hover:text-fg"
      >
        {copied ? <Check className="size-3" /> : <Copy className="size-3" />}
        {copied ? "Copied" : "Copy"}
      </button>
      <pre className="p-5 font-mono text-[12px] leading-relaxed text-fg">{text}</pre>
    </div>
  );
}
