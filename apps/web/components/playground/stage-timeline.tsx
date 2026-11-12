"use client";

import { Check, Minus, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { STAGE_ORDER, type StageName, type Stages } from "@/lib/playground";

const COPY: Record<StageName, { title: string; idle: string }> = {
  route: { title: "Route", idle: "Find the integrations that could serve this request" },
  map: { title: "Map", idle: "Read each one's actions and keep the plausible ones" },
  reduce: { title: "Reduce", idle: "Write the graph from those actions' full schemas" },
  validate: { title: "Validate", idle: "Check structure, node types and every reference" },
  repair: { title: "Repair", idle: "Send any validator errors back to the model" },
};

function seconds(ms: number) {
  return ms < 1000 ? `${ms} ms` : `${(ms / 1000).toFixed(1)} s`;
}

export function StageTimeline({ stages, finished }: { stages: Stages; finished: boolean }) {
  return (
    <ol className="relative" aria-label="Compile stages">
      {STAGE_ORDER.map((name, i) => {
        const stage = stages[name];
        const skipped = name === "repair" && finished && stage.status === "idle";
        const items = stage.items ?? [];
        return (
          <li key={name} className="relative flex gap-3 pb-4 last:pb-0">
            {i < STAGE_ORDER.length - 1 && (
              <span aria-hidden className="absolute left-[9px] top-6 bottom-0 w-px bg-border" />
            )}
            <StageIcon status={skipped ? "skipped" : stage.status} />
            <div className="min-w-0 flex-1">
              <div className="flex items-baseline justify-between gap-2">
                <span className={cn("text-[13px] font-medium", stage.status === "idle" && !skipped ? "text-muted" : "text-fg")}>
                  {COPY[name].title}
                </span>
                {stage.durationMs > 0 && (
                  <span className="font-mono text-[11px] tabular-nums text-faint">{seconds(stage.durationMs)}</span>
                )}
              </div>
              <p className="mt-0.5 text-[12px] leading-relaxed text-muted">
                {skipped ? "Not needed. The first graph passed." : (stage.detail ?? COPY[name].idle)}
              </p>
              {items.length > 0 && (
                <ul className="mt-1.5 flex flex-wrap gap-1">
                  {items.slice(0, name === "validate" ? 4 : 8).map((item) => (
                    <li
                      key={item}
                      className={cn(
                        "max-w-full truncate rounded px-1.5 py-0.5 font-mono text-[10.5px]",
                        name === "validate" ? "bg-warn-bg text-warn" : "bg-surface-2 text-muted",
                      )}
                      title={item}
                    >
                      {item.replace(/^pieces\./, "")}
                    </li>
                  ))}
                  {items.length > 8 && name !== "validate" && (
                    <li className="px-1 py-0.5 text-[10.5px] text-faint">+{items.length - 8} more</li>
                  )}
                </ul>
              )}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function StageIcon({ status }: { status: "idle" | "running" | "done" | "failed" | "skipped" }) {
  const base = "relative z-10 mt-0.5 flex size-[18px] shrink-0 items-center justify-center rounded-full border bg-surface";
  if (status === "done") {
    return (
      <span className={cn(base, "border-success bg-success text-white")}>
        <Check className="size-3" strokeWidth={3} />
      </span>
    );
  }
  if (status === "failed") {
    return (
      <span className={cn(base, "border-warn bg-warn text-white")}>
        <X className="size-3" strokeWidth={3} />
      </span>
    );
  }
  if (status === "running") {
    return (
      <span className={cn(base, "border-accent")}>
        <span className="live-dot size-2 rounded-full bg-accent" />
      </span>
    );
  }
  if (status === "skipped") {
    return (
      <span className={cn(base, "border-border-strong text-faint")}>
        <Minus className="size-3" />
      </span>
    );
  }
  return <span className={cn(base, "border-border-strong")} />;
}
