"use client";

import { X } from "lucide-react";
import { splitRefs, type PublicNode } from "@/lib/playground";

function asText(value: unknown): string {
  if (typeof value === "string") return value;
  return JSON.stringify(value, null, 2);
}

function refTarget(ref: string, nodes: PublicNode[]): string | null {
  const [root, key] = ref.split(".");
  if (root !== "outputs" || !key) return null;
  return nodes.some((n) => n.key === key) ? key : null;
}

export function NodeInspector({
  node,
  nodes,
  onSelect,
}: {
  node: PublicNode;
  nodes: PublicNode[];
  onSelect: (key: string | null) => void;
}) {
  const config = Object.entries(node.config ?? {}).filter(([, v]) => v !== "" && v !== null);
  const deps = node.dependsOn ?? [];
  const outputs = node.outputs ?? [];
  return (
    <aside className="flex h-full flex-col overflow-hidden" aria-label={`Details for ${node.name}`}>
      <div className="flex items-start justify-between gap-3 border-b border-border px-4 py-3">
        <div className="min-w-0">
          <h3 className="truncate text-[14px] font-semibold text-fg">{node.name || node.key}</h3>
          <p className="mt-0.5 break-all font-mono text-[11px] text-faint">{node.type}</p>
        </div>
        <button
          onClick={() => onSelect(null)}
          aria-label="Close details"
          className="rounded p-1 text-faint transition-colors hover:bg-surface-2 hover:text-fg"
        >
          <X className="size-4" />
        </button>
      </div>
      <div className="flex-1 space-y-5 overflow-y-auto px-4 py-4">
        {node.service && (
          <Section title="Catalog action">
            <p className="text-[12.5px] text-muted">
              {node.service}: {node.label}
            </p>
          </Section>
        )}
        {deps.length > 0 && (
          <Section title="Runs after">
            <div className="flex flex-wrap gap-1">
              {deps.map((d) => (
                <button
                  key={d}
                  onClick={() => onSelect(d)}
                  className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[11px] text-muted transition-colors hover:text-fg"
                >
                  {d}
                </button>
              ))}
            </div>
          </Section>
        )}
        <Section title="Configuration">
          {config.length === 0 ? (
            <p className="text-[12.5px] text-faint">No configuration needed.</p>
          ) : (
            <dl className="space-y-3">
              {config.map(([key, value]) => (
                <div key={key}>
                  <dt className="font-mono text-[11px] text-faint">{key}</dt>
                  <dd className="mt-0.5 whitespace-pre-wrap break-words text-[12.5px] leading-relaxed text-fg">
                    {splitRefs(asText(value)).map((part, i) => {
                      if (!part.ref) return <span key={i}>{part.text}</span>;
                      const target = refTarget(part.ref, nodes);
                      return target ? (
                        <button
                          key={i}
                          onClick={() => onSelect(target)}
                          title={`Go to ${target}`}
                          className="rounded bg-accent-bg px-1 font-mono text-[11.5px] text-accent-soft hover:underline"
                        >
                          {part.text}
                        </button>
                      ) : (
                        <span key={i} className="rounded bg-surface-2 px-1 font-mono text-[11.5px] text-muted">
                          {part.text}
                        </span>
                      );
                    })}
                  </dd>
                </div>
              ))}
            </dl>
          )}
        </Section>
        {outputs.length > 0 && (
          <Section title="Declared outputs">
            <div className="flex flex-wrap gap-1">
              {outputs.map((o) => (
                <span key={o} className="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[11px] text-muted">
                  {o}
                </span>
              ))}
            </div>
          </Section>
        )}
      </div>
    </aside>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section>
      <h4 className="mb-1.5 text-[11px] font-medium uppercase tracking-wider text-faint">{title}</h4>
      {children}
    </section>
  );
}
