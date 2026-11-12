import Link from "next/link";
import type { Metadata } from "next";
import { ArrowRight } from "lucide-react";
import { REPO_URL, SiteNav, Wordmark } from "@/components/playground/site-nav";

export const metadata: Metadata = {
  title: "Apex: prompts in, durable workflows out",
  description:
    "Apex compiles a plain-English request into a validated workflow graph over 6,758 real integration actions, then runs it durably on Temporal.",
};

const FUNNEL = [
  { n: "6,758", label: "actions and triggers", note: "from 707 integrations" },
  { n: "100+", label: "integrations", note: "kept by embedding search" },
  { n: "≤ 16", label: "integrations", note: "picked by the router" },
  { n: "≤ 48", label: "action schemas", note: "given to the graph author" },
  { n: "1", label: "validated graph", note: "or a clear failure" },
];

const GUARANTEES = [
  {
    title: "It can only use what exists",
    body: "Every node type comes from a closed catalog. A model that invents an action doesn't get a broken step, it gets a validation error naming the type.",
  },
  {
    title: "Data wiring is type-checked",
    body: "A reference like {{outputs.fetch.items}} is checked against the outputs that upstream node declares. A field that doesn't exist never reaches a run.",
  },
  {
    title: "Mistakes go back to the model",
    body: "Validator errors are returned verbatim for up to two repair rounds. Large graphs are written section by section, so an error in section 5 doesn't rewrite sections 1 to 4.",
  },
  {
    title: "It never fakes success",
    body: "There's no fallback graph and no mock output. If a stage can't produce something valid, the compile fails and says why.",
  },
];

const HISTORY = [
  { node: "fetch_charges", state: "Completed", tone: "text-success", note: "read from history" },
  { node: "summarize", state: "Completed", tone: "text-success", note: "read from history" },
  { node: "post_to_slack", state: "Worker killed", tone: "text-warn", note: "retried on restart" },
  { node: "add_sheet_row", state: "Waiting", tone: "text-faint", note: "runs next" },
];

export default function LandingPage() {
  return (
    <div className="min-h-dvh bg-bg">
      <SiteNav />
      <main>
        <section className="mx-auto max-w-[1040px] px-5 pb-16 pt-16 md:pt-24">
          <h1 className="max-w-[780px] text-balance text-[40px] font-semibold leading-[1.05] tracking-[-0.03em] text-fg md:text-[60px]">
            Describe the work. Get a workflow that actually runs.
          </h1>
          <p className="mt-6 max-w-[620px] text-[17px] leading-relaxed text-muted">
            Apex is a compiler for automations. It turns one sentence into a typed execution graph over thousands
            of real integration actions, checks every step before anything is saved, and runs it so a crash
            resumes at the exact node.
          </p>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Link
              href="/playground"
              className="flex h-11 items-center gap-2 rounded-md bg-accent px-5 text-[15px] font-medium text-accent-fg transition-colors hover:bg-accent-soft"
            >
              Compile a workflow <ArrowRight className="size-4" />
            </Link>
            <a
              href={REPO_URL}
              rel="noreferrer"
              className="flex h-11 items-center rounded-md border border-border-strong px-5 text-[15px] font-medium text-fg transition-colors hover:bg-surface-2"
            >
              Read the code
            </a>
          </div>
        </section>

        <section className="border-y border-border bg-inset" aria-labelledby="funnel">
          <div className="mx-auto max-w-[1040px] px-5 py-14">
            <h2 id="funnel" className="text-[22px] font-semibold tracking-tight text-fg">
              No model can read 6,758 schemas. So the compiler narrows them.
            </h2>
            <p className="mt-2 max-w-[620px] text-[15px] leading-relaxed text-muted">
              Each stage is cheap enough to cast wide and strict enough that the expensive model only sees what matters.
            </p>
            <ol className="mt-9 grid grid-cols-2 gap-px overflow-hidden rounded-lg border border-border bg-border md:grid-cols-5">
              {FUNNEL.map((step) => (
                <li key={step.note} className="bg-bg p-4 last:col-span-2 md:last:col-span-1">
                  <div className="font-mono text-[28px] font-medium tabular-nums tracking-tight text-fg">{step.n}</div>
                  <div className="mt-1 text-[13px] font-medium text-fg">{step.label}</div>
                  <div className="mt-0.5 text-[12px] leading-snug text-muted">{step.note}</div>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section className="mx-auto max-w-[1040px] px-5 py-16" aria-labelledby="trust">
          <h2 id="trust" className="text-[22px] font-semibold tracking-tight text-fg">
            The hard part is not trusting the model
          </h2>
          <div className="mt-8 grid gap-x-12 gap-y-8 md:grid-cols-2">
            {GUARANTEES.map((g) => (
              <div key={g.title}>
                <h3 className="text-[15px] font-medium text-fg">{g.title}</h3>
                <p className="mt-1.5 text-[14px] leading-relaxed text-muted">{g.body}</p>
              </div>
            ))}
          </div>
        </section>

        <section className="border-y border-border bg-inset" aria-labelledby="durable">
          <div className="mx-auto grid max-w-[1040px] items-center gap-10 px-5 py-16 md:grid-cols-2">
            <div>
              <h2 id="durable" className="text-[22px] font-semibold tracking-tight text-fg">
                Kill the worker. The run keeps its place.
              </h2>
              <p className="mt-3 text-[15px] leading-relaxed text-muted">
                Every node is its own Temporal activity, and independent branches run in parallel. When a worker dies,
                completed nodes are read from workflow history instead of being called again, so a restart doesn&apos;t
                re-send the email or re-charge the card.
              </p>
            </div>
            <table className="w-full overflow-hidden rounded-lg border border-border bg-bg text-[13px]">
              <caption className="sr-only">How a run recovers after a worker crash</caption>
              <tbody>
                {HISTORY.map((row) => (
                  <tr key={row.node} className="border-b border-border last:border-0">
                    <td className="px-4 py-3 font-mono text-[12px] text-fg">{row.node}</td>
                    <td className={`px-2 py-3 font-medium ${row.tone}`}>{row.state}</td>
                    <td className="hidden px-4 py-3 text-right text-muted sm:table-cell">{row.note}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        <section className="mx-auto max-w-[1040px] px-5 py-20">
          <h2 className="text-[28px] font-semibold tracking-tight text-fg">Try to break it.</h2>
          <p className="mt-2 max-w-[520px] text-[15px] leading-relaxed text-muted">
            The compiler is live. No account needed: describe something messy and watch every stage work.
          </p>
          <Link
            href="/playground"
            className="mt-6 inline-flex h-11 items-center gap-2 rounded-md bg-accent px-5 text-[15px] font-medium text-accent-fg transition-colors hover:bg-accent-soft"
          >
            Open the playground <ArrowRight className="size-4" />
          </Link>
        </section>
      </main>
      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-[1040px] flex-wrap items-center justify-between gap-3 px-5 py-6 text-[13px] text-muted">
          <Wordmark />
          <span>
            Integrations from{" "}
            <a className="underline hover:text-fg" href="https://github.com/activepieces/activepieces" rel="noreferrer">
              Activepieces
            </a>{" "}
            (MIT)
          </span>
        </div>
      </footer>
    </div>
  );
}
