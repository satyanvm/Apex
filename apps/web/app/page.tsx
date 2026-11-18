import Link from "next/link";
import {
  ArrowRight,
  Plug,
  Network,
  Globe,
  Terminal,
  Gauge,
  Bell,
  ShieldCheck,
  Workflow as WorkflowIcon,
  Eye,
} from "lucide-react";
import { StatusBadge, StatusDot } from "@/components/ui/status";
import { LivePill } from "@/components/live";
import { ThemeToggle } from "@/components/ui/theme-toggle";
import { AccountNav } from "@/components/account-nav";
import { Wordmark } from "@/components/playground/site-nav";

/* Editorial theme: mint grid canvas, navy ink, display serif headlines,
   typewriter labels, square controls and flat colour blocks (globals.css). */

const MINT_TEXT = "text-[#e2f9d0]";

export default function LandingPage() {
  return (
    <div className="relative overflow-x-clip bg-bg">
      <LandingNav />
      <Hero />
      <LogoCloud />
      <Problem />
      <HowItWorks />
      <Features />
      <Metrics />
      <CTA />
      <Footer />
    </div>
  );
}

/* ----------------------------------------------------------------- Nav */
function LandingNav() {
  return (
    <header className="sticky top-0 z-40 border-b border-border bg-surface">
      <div className="mx-auto flex h-16 max-w-[1280px] items-center gap-10 px-6">
        <Wordmark />
        <nav className="hidden items-center gap-8 text-[15px] font-medium text-fg md:flex">
          <a className="transition-colors hover:text-accent" href="#product">Product</a>
          <a className="transition-colors hover:text-accent" href="#how">How it works</a>
          <a className="transition-colors hover:text-accent" href="#features">Features</a>
          <a className="transition-colors hover:text-accent" href="#">Pricing</a>
          <a className="transition-colors hover:text-accent" href="#">Docs</a>
        </nav>
        <div className="ml-auto flex items-center gap-2">
          <ThemeToggle />
          <AccountNav />
        </div>
      </div>
    </header>
  );
}

/* ---------------------------------------------------------------- Hero */
function Hero() {
  return (
    <section className="relative overflow-hidden border-b border-border bg-mint bg-grid dark:bg-bg">
      {/* Stepped lighter blocks in the corners, cut from the same grid. */}
      <div className="pointer-events-none absolute inset-0" aria-hidden>
        <span className="absolute left-0 top-0 h-20 w-56 bg-[#d2f4bb] dark:bg-surface" />
        <span className="absolute left-0 top-20 h-24 w-24 bg-[#d2f4bb] dark:bg-surface" />
        <span className="absolute right-0 top-0 h-24 w-64 bg-[#d2f4bb] dark:bg-surface" />
      </div>

      <div className="relative mx-auto grid max-w-[1280px] grid-cols-1 items-center gap-14 px-6 pb-20 pt-16 md:pt-24 lg:grid-cols-[1.05fr_1fr]">
        <div className="animate-in min-w-0">
          <span className="eyebrow">The cloud for autonomous agents · v2 is live</span>

          <h1 className="font-display mt-6 text-balance text-[44px] font-semibold leading-[0.95] text-fg sm:text-[68px] lg:text-[84px]">
            Deploy agents that keep running after you leave.
          </h1>

          <p className="mt-7 max-w-[560px] text-[17px] leading-relaxed text-fg md:text-[18px]">
            Apex is the execution platform for long-running, multi-agent
            workflows. Launch a crew, close your laptop, and come back to finished
            work — with live logs, browser activity, cost and runtime in one
            control plane.
          </p>

          <div className="mt-9 flex flex-col gap-3 sm:flex-row">
            <Link
              href="/workflows/competitive-intelligence-sweep"
              className="flex h-14 items-center justify-center gap-2 border-2 border-fg px-7 text-[17px] font-medium text-fg transition-colors hover:bg-fg hover:text-bg"
            >
              <Eye className="size-[18px]" />
              See a live workflow
            </Link>
            <Link
              href="/signup?next=%2Fdashboard"
              className="group flex h-14 items-center justify-center gap-2 bg-accent px-7 text-[17px] font-medium text-accent-fg transition-colors hover:bg-accent-soft"
            >
              Start building
              <ArrowRight className="size-[18px] transition-transform group-hover:translate-x-0.5" />
            </Link>
          </div>
          <p className="font-typewriter mt-5 text-[12.5px] text-muted">
            No credit card · 100 agent-hours free · SOC 2 Type II
          </p>
        </div>

        <HeroCollage />
      </div>

      {/* Cobalt grid strip along the bottom edge. */}
      <div className="pattern-grid-cobalt pointer-events-none absolute bottom-0 left-6 h-6 w-[44%]" aria-hidden />
    </section>
  );
}

/* The product window, set into a collage of flat colour blocks. */
function HeroCollage() {
  return (
    <div className="animate-in relative mx-auto w-full min-w-0 max-w-[620px] pb-10 pt-16 lg:pt-10">
      <div className="pattern-grid-forest absolute -right-8 top-6 h-[72%] w-[62%]" aria-hidden />
      <div
        className="absolute -right-2 top-0 h-28 w-32 bg-lime"
        style={{ clipPath: "polygon(30% 0, 100% 62%, 0 100%)" }}
        aria-hidden
      />
      <div className="pattern-dots-pink absolute -left-8 bottom-0 h-44 w-44 rotate-45" aria-hidden />

      {/* Display word on a highlight slip, with a typed tag. */}
      <div className="absolute left-2 top-2 z-20 sm:left-10">
        <span className="font-typewriter absolute -top-3 right-0 translate-x-1/3 bg-surface px-2 py-0.5 text-[13px] font-medium text-fg">
          Agent crew
        </span>
        <span className="font-display block bg-[#ffc2e6] px-3 pb-1 text-[44px] leading-none text-pink sm:text-[56px]">
          Durable
        </span>
      </div>

      <div className="pattern-grid-blush relative z-10 mt-10 p-3 sm:p-4">
        <HeroPreview />
      </div>
    </div>
  );
}

/* Faux product window — gives the page its "this is real" weight. */
function HeroPreview() {
  return (
    <div className="min-w-0 overflow-hidden border-2 border-fg bg-surface text-left">
      {/* window chrome */}
      <div className="flex items-center gap-2 border-b-2 border-fg bg-surface-2 px-3 py-2">
        <span className="h-2.5 w-2.5 bg-accent" />
        <span className="h-2.5 w-2.5 bg-lime" />
        <span className="h-2.5 w-2.5 bg-pink" />
        <div className="font-typewriter ml-2 flex items-center gap-1.5 bg-surface px-2 py-0.5 text-[11px] text-muted">
          <ShieldCheck className="size-3 text-success" />
          apex/runs/142
        </div>
        <div className="ml-auto"><LivePill /></div>
      </div>
      <div className="p-4">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div className="min-w-0">
            <div className="truncate text-[13px] font-semibold">Competitive Intelligence Sweep</div>
            <div className="font-typewriter text-[11px] text-faint">Run #142 · 7 agents · us-east-1</div>
          </div>
          <StatusBadge status="running" />
        </div>
        {/* agent rail */}
        <div className="mb-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
          {[
            { n: "Conductor", s: "running" as const },
            { n: "Scout · Pricing", s: "succeeded" as const },
            { n: "Navigator", s: "running" as const },
            { n: "Synthesizer", s: "waiting" as const },
          ].map((a) => (
            <div key={a.n} className="border border-border-strong bg-surface p-2">
              <div className="mb-2 flex items-center justify-between gap-1">
                <span className="truncate text-[11px] font-medium">{a.n}</span>
                <StatusDot status={a.s} />
              </div>
              <div className="h-1.5 w-full bg-surface-3">
                <div
                  className="h-full bg-accent"
                  style={{ width: a.s === "succeeded" ? "100%" : a.s === "waiting" ? "12%" : "64%" }}
                />
              </div>
            </div>
          ))}
        </div>
        {/* faux log */}
        <div className="border border-border-strong bg-inset p-3 font-mono text-[10.5px] leading-relaxed">
          <LogRow t="17:09:12" c="text-faint" s="system" m="Sandbox ready · 4 vCPU / 8 GiB" />
          <LogRow t="17:09:52" c="text-blue-600" s="Conductor" m="→ delegated 3 scouts" />
          <LogRow t="17:11:31" c="text-emerald-600" s="Navigator" m="screenshot captured · acme-pricing.png" />
          <LogRow t="17:14:30" c="text-success" s="Scout·Pricing" m="completed · 11/11 rows" />
          <LogRow t="17:18:10" c="text-warn" s="Synthesizer" m="price conflict flagged for audit" />
          <div className="flex min-w-0 gap-2 text-fg">
            <span className="text-ghost">17:31:11</span>
            <span className="text-accent">Composer</span>
            <span>drafting executive brief<span className="caret text-accent">▍</span></span>
          </div>
        </div>
      </div>
    </div>
  );
}

function LogRow({ t, c, s, m }: { t: string; c: string; s: string; m: string }) {
  return (
    <div className="flex min-w-0 gap-2">
      <span className="shrink-0 text-ghost">{t}</span>
      <span className={`shrink-0 ${c}`}>{s}</span>
      <span className="min-w-0 truncate text-muted">{m}</span>
    </div>
  );
}

/* ------------------------------------------------------------ Logo cloud */
function LogoCloud() {
  return (
    <section className="border-b border-border bg-surface py-12">
      <div className="mx-auto max-w-[1280px] px-6 text-center">
        <p className="font-typewriter mb-7 text-[13px] text-muted">Trusted to run autonomous work at</p>
        <div className="flex flex-wrap items-center justify-center gap-x-14 gap-y-6">
          {["Northwind", "Helix", "Quanta", "Vantage", "Orbital", "Meridian"].map((b) => (
            <span key={b} className="font-display text-[26px] font-semibold text-fg/70">
              {b}
            </span>
          ))}
        </div>
      </div>
    </section>
  );
}

/* -------------------------------------------------------------- Problem */
function Problem() {
  return (
    <section id="product" className="mx-auto max-w-[1280px] px-6 py-24">
      <div className="grid items-center gap-14 md:grid-cols-2">
        <div>
          <span className="eyebrow">The problem</span>
          <h2 className="font-display mt-5 text-balance text-[40px] font-semibold leading-[1] md:text-[56px]">
            Agents are powerful. The place they run is not.
          </h2>
          <p className="mt-6 text-[16px] leading-relaxed text-muted">
            Today, autonomous agents live inside a notebook, a terminal tab, or a
            script on your laptop. The moment you disconnect, they die. There&apos;s
            no durable execution, no visibility into what a 7-agent crew is doing,
            no record of which browser it drove or what it spent.
          </p>
          <ul className="mt-7 space-y-3">
            {[
              "Long jobs stop the second the tab closes",
              "No way to watch a multi-agent crew coordinate",
              "Browser steps fail silently, with no replay",
              "Cost and runtime are a mystery until the bill arrives",
            ].map((t) => (
              <li key={t} className="flex items-start gap-3 text-[15px] text-fg">
                <span className="mt-2 h-2 w-2 shrink-0 bg-accent" />
                {t}
              </li>
            ))}
          </ul>
        </div>

        <div className="relative">
          <div className="border-2 border-fg bg-surface p-6">
            <div className="mb-4 flex items-center justify-between">
              <span className="font-typewriter text-[13px] text-muted">Without a platform</span>
              <span className="bg-danger-bg px-2 py-0.5 text-[11px] text-danger">disconnected</span>
            </div>
            <div className="space-y-2 font-mono text-[12px]">
              <div className="text-faint">$ python run_agents.py</div>
              <div className="text-muted">[agent] researching competitors…</div>
              <div className="text-muted">[agent] browsing acme.dev…</div>
              <div className="text-danger">✗ Terminated (SIGHUP) — session closed</div>
              <div className="text-ghost">work lost · no logs · no artifacts</div>
            </div>
          </div>
          <div className="pattern-grid-forest mt-5 border-2 border-fg p-6 shadow-pop">
            <div className="mb-4 flex items-center justify-between">
              <span className={`font-typewriter text-[13px] ${MINT_TEXT}`}>With Apex</span>
              <span className="bg-lime px-2 py-0.5 text-[11px] font-medium text-navy">still running</span>
            </div>
            <div className="space-y-2 font-mono text-[12px]">
              <div className="text-[#e2f9d0]/60">$ apex deploy ./crew</div>
              <div className="text-lime">✓ Crew running in us-east-1 · detached</div>
              <div className={MINT_TEXT}>7 agents · live logs · browser capture</div>
              <div className="text-[#ffb3e6]">→ notified when complete</div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------ How it works */
function HowItWorks() {
  const steps = [
    {
      icon: WorkflowIcon,
      title: "Define your workflow",
      body: "Describe a crew of agents, their dependencies, and the tools they can use. Version every change.",
    },
    {
      icon: Plug,
      title: "Deploy & detach",
      body: "We provision an isolated sandbox per run and execute durably. Close your laptop — it keeps going.",
    },
    {
      icon: Gauge,
      title: "Monitor live",
      body: "Watch the agent graph coordinate, stream logs, scrub browser sessions, and track cost in real time.",
    },
    {
      icon: Bell,
      title: "Get the result",
      body: "Artifacts are collected and you're notified the moment a run completes, fails, or needs a human.",
    },
  ];
  return (
    <section id="how" className="bg-navy py-24">
      <div className="mx-auto max-w-[1280px] px-6">
        <div className="mb-14">
          <span className="eyebrow">How it works</span>
          <h2 className={`font-display mt-5 max-w-[760px] text-balance text-[40px] font-semibold leading-[1] md:text-[56px] ${MINT_TEXT}`}>
            From prompt to finished work, durably.
          </h2>
        </div>
        <div className="grid gap-px bg-[#e2f9d0]/20 md:grid-cols-4">
          {steps.map((s, i) => {
            const Icon = s.icon;
            return (
              <div key={s.title} className="relative bg-navy p-6">
                <span className="font-typewriter absolute right-5 top-5 text-[13px] text-lime">0{i + 1}</span>
                <span className="mb-5 flex h-11 w-11 items-center justify-center bg-lime">
                  <Icon className="size-5 text-navy" />
                </span>
                <h3 className={`font-display text-[22px] font-semibold leading-tight ${MINT_TEXT}`}>{s.title}</h3>
                <p className="mt-3 text-[14px] leading-relaxed text-[#e2f9d0]/75">{s.body}</p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

/* ------------------------------------------------------------- Features */
function Features() {
  const features = [
    { icon: Network, title: "Multi-agent graphs", body: "Visualize how a crew coordinates — dependencies, hand-offs, and live status for every agent." },
    { icon: Terminal, title: "World-class log streaming", body: "Realtime, searchable, filterable logs with reasoning traces, tool calls, and browser events on one timeline." },
    { icon: Globe, title: "Browser session replay", body: "Every navigation, click, and screenshot captured. Scrub the timeline and see exactly what the agent saw." },
    { icon: Gauge, title: "Cost & runtime tracking", body: "Per-run, per-agent token and dollar accounting with budgets and alerts before you overspend." },
    { icon: ShieldCheck, title: "Isolated sandboxes", body: "Each run executes in its own ephemeral, hardened sandbox. SOC 2 Type II, region-pinned." },
    { icon: Bell, title: "Notifications", body: "Completed, failed, browser errors, cost thresholds — delivered to Slack, email, or webhook." },
  ];
  return (
    <section id="features" className="bg-surface">
      <div className="mx-auto max-w-[1280px] px-6 py-24">
        <div className="mb-14 max-w-[720px]">
          <span className="eyebrow border border-border-strong">The control plane</span>
          <h2 className="font-display mt-5 text-balance text-[40px] font-semibold leading-[1] md:text-[56px]">
            Everything you need to run agents in production.
          </h2>
          <p className="mt-5 text-[16px] leading-relaxed text-muted">
            One console for launching, observing, and debugging autonomous work —
            built for the density of real operations and the polish of a tool you
            live in all day.
          </p>
        </div>
        <div className="grid gap-px border-2 border-fg bg-fg md:grid-cols-3">
          {features.map((f) => {
            const Icon = f.icon;
            return (
              <div key={f.title} className="group bg-surface p-7 transition-colors hover:bg-mint dark:hover:bg-surface-2">
                <span className="mb-5 flex h-11 w-11 items-center justify-center border-2 border-fg bg-surface transition-colors group-hover:bg-accent group-hover:border-accent">
                  <Icon className="size-5 text-fg transition-colors group-hover:text-accent-fg" />
                </span>
                <h3 className="font-display text-[22px] font-semibold leading-tight">{f.title}</h3>
                <p className="mt-2.5 text-[14px] leading-relaxed text-muted">{f.body}</p>
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

/* -------------------------------------------------------------- Metrics */
function Metrics() {
  const stats = [
    { v: "99.98%", l: "execution uptime" },
    { v: "2.1M+", l: "agent-hours run" },
    { v: "180ms", l: "log stream latency" },
    { v: "14", l: "regions worldwide" },
  ];
  return (
    <section className="pattern-grid-forest py-20">
      <div className="mx-auto grid max-w-[1280px] grid-cols-2 gap-10 px-6 md:grid-cols-4">
        {stats.map((s) => (
          <div key={s.l}>
            <div className="font-display text-[48px] font-semibold leading-none text-lime md:text-[60px]">{s.v}</div>
            <div className={`font-typewriter mt-3 text-[13px] ${MINT_TEXT}`}>{s.l}</div>
          </div>
        ))}
      </div>
    </section>
  );
}

/* ------------------------------------------------------------------ CTA */
function CTA() {
  return (
    <section className="bg-mint bg-grid dark:bg-bg">
      <div className="mx-auto max-w-[1280px] px-6 py-28">
        <div className="pattern-grid-blush relative border-2 border-fg px-8 py-16 shadow-pop md:px-16">
          <div className="pattern-dots-pink absolute -right-6 -top-6 hidden h-20 w-20 rotate-45 md:block" aria-hidden />
          <h2 className="font-display max-w-[720px] text-balance text-[40px] font-semibold leading-[1] text-navy md:text-[60px]">
            Give your agents a place to live.
          </h2>
          <p className="mt-5 max-w-[520px] text-[17px] text-navy">
            Deploy your first autonomous workflow in minutes. Watch it run long
            after you&apos;ve logged off.
          </p>
          <div className="mt-9 flex flex-col gap-3 sm:flex-row">
            <Link
              href="/signup?next=%2Fdashboard"
              className="group flex h-14 items-center justify-center gap-2 bg-accent px-7 text-[17px] font-medium text-accent-fg transition-colors hover:bg-accent-soft"
            >
              Open the console
              <ArrowRight className="size-[18px] transition-transform group-hover:translate-x-0.5" />
            </Link>
            <Link
              href="#"
              className="flex h-14 items-center justify-center border-2 border-navy px-7 text-[17px] font-medium text-navy transition-colors hover:bg-navy hover:text-[#e2f9d0]"
            >
              Talk to sales
            </Link>
          </div>
        </div>
      </div>
    </section>
  );
}

/* --------------------------------------------------------------- Footer */
function Footer() {
  const cols: { h: string; links: string[] }[] = [
    { h: "Product", links: ["Workflows", "Agents", "Browser sessions", "Pricing"] },
    { h: "Developers", links: ["Documentation", "API reference", "CLI", "Status"] },
    { h: "Company", links: ["About", "Careers", "Blog", "Security"] },
  ];
  return (
    <footer className="bg-navy">
      <div className="mx-auto grid max-w-[1280px] gap-10 px-6 py-16 md:grid-cols-[1.4fr_1fr_1fr_1fr]">
        <div>
          <span className={`font-display text-[34px] font-semibold ${MINT_TEXT}`}>Apex</span>
          <p className="mt-3 max-w-[280px] text-[14px] leading-relaxed text-[#e2f9d0]/70">
            The execution platform for long-running, autonomous AI agents.
          </p>
        </div>
        {cols.map((c) => (
          <div key={c.h}>
            <h4 className="font-typewriter mb-4 text-[13px] text-lime">{c.h}</h4>
            <ul className="space-y-2.5">
              {c.links.map((l) => (
                <li key={l}>
                  <a href="#" className="text-[14px] text-[#e2f9d0]/80 transition-colors hover:text-[#e2f9d0]">{l}</a>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <div className="border-t border-[#e2f9d0]/15">
        <div className="mx-auto flex max-w-[1280px] flex-col items-center justify-between gap-3 px-6 py-5 text-[12px] text-[#e2f9d0]/60 sm:flex-row">
          <span className="font-typewriter">© 2026 Apex. All rights reserved.</span>
          <div className="flex items-center gap-5">
            <a href="#" className="transition-colors hover:text-[#e2f9d0]">Privacy</a>
            <a href="#" className="transition-colors hover:text-[#e2f9d0]">Terms</a>
            <a href="#" className="transition-colors hover:text-[#e2f9d0]">SOC 2</a>
          </div>
        </div>
      </div>
    </footer>
  );
}
