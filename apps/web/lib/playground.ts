export type StageName = "route" | "map" | "reduce" | "validate" | "repair";

export type CompileEvent = {
  stage: StageName;
  status: "start" | "done" | "failed";
  detail?: string;
  items?: string[];
  count?: number;
  total?: number;
  attempt?: number;
  durationMs?: number;
};

export type PublicNode = {
  key: string;
  name: string;
  type: string;
  label?: string;
  service?: string;
  kind?: string;
  dependsOn: string[] | null;
  config: Record<string, unknown> | null;
  outputs?: string[];
  col: number;
  row: number;
};

export type CompileResult = {
  name: string;
  description: string;
  schedule: string | null;
  defaultInput: Record<string, unknown> | null;
  sources: string[] | null;
  nodes: PublicNode[];
  durationMs: number;
  repairs: number;
  cached: boolean;
};

export type StageState = {
  status: "idle" | "running" | "done" | "failed";
  detail?: string;
  items?: string[];
  durationMs: number;
  rounds: number;
};

export type Stages = Record<StageName, StageState>;

export const STAGE_ORDER: StageName[] = ["route", "map", "reduce", "validate", "repair"];

export function emptyStages(): Stages {
  const idle = (): StageState => ({ status: "idle", durationMs: 0, rounds: 0 });
  return { route: idle(), map: idle(), reduce: idle(), validate: idle(), repair: idle() };
}

export function applyEvent(stages: Stages, ev: CompileEvent): Stages {
  const prev = stages[ev.stage];
  if (!prev) return stages;
  const next: StageState = { ...prev };
  if (ev.status === "start") {
    next.status = "running";
    next.rounds = prev.rounds + (ev.stage === "repair" ? 1 : 0);
  } else {
    next.status = ev.status;
    next.durationMs = prev.durationMs + (ev.durationMs ?? 0);
  }
  if (ev.detail) next.detail = ev.detail;
  if (ev.items) next.items = ev.items;
  return { ...stages, [ev.stage]: next };
}

/** Parses one SSE frame into its event name and JSON payload. */
export function parseFrame(frame: string): { event: string; data: unknown } | null {
  let event = "message";
  const data: string[] = [];
  for (const line of frame.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data.push(line.slice(5).trim());
  }
  if (data.length === 0) return null;
  try {
    return { event, data: JSON.parse(data.join("\n")) };
  } catch {
    return null;
  }
}

/** Splits a config value around {{…}} references. */
export function splitRefs(text: string): Array<{ text: string; ref?: string }> {
  const parts: Array<{ text: string; ref?: string }> = [];
  const pattern = /\{\{\s*([^}]+?)\s*\}\}/g;
  let last = 0;
  for (const m of text.matchAll(pattern)) {
    const at = m.index ?? 0;
    if (at > last) parts.push({ text: text.slice(last, at) });
    parts.push({ text: m[0], ref: m[1] });
    last = at + m[0].length;
  }
  if (last < text.length) parts.push({ text: text.slice(last) });
  return parts;
}

export function kindOf(node: PublicNode): "trigger" | "agent" | "tool" | "logic" | "output" | "app" {
  if (node.kind === "trigger") return "trigger";
  const prefix = node.type.split(".")[0];
  if (prefix === "trigger" || prefix === "agent" || prefix === "tool" || prefix === "logic" || prefix === "output") {
    return prefix;
  }
  return "app";
}

export const EXAMPLES = [
  {
    label: "Revenue digest",
    prompt:
      "Every weekday at 9am, pull new Stripe charges over $500, summarize them with AI, post the summary to the #finance Slack channel, and add a row for each charge to a Google Sheet.",
  },
  {
    label: "Bug triage",
    prompt:
      "When a GitHub issue is labeled bug, search Sentry for matching errors, have AI draft a triage note with the likely cause, and create a Linear ticket with that note.",
  },
  {
    label: "Lead routing",
    prompt:
      "When someone submits our Typeform, have AI score the lead from 0 to 100. If the score is above 70, create a HubSpot deal and alert sales on Slack. Otherwise add them to a Mailchimp nurture list.",
  },
  {
    label: "Competitor watch",
    prompt:
      "Every Monday, browse three competitors' pricing pages in parallel, have AI compare them against last week's notes in Notion, and email the founders a report.",
  },
];
