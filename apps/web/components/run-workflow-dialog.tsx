"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Play, Loader2 } from "lucide-react";
import { Dialog, DialogHeader } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { fetchPreflight, launchRun, type RunPreflight } from "@/lib/api";
import { ConnectAccounts } from "@/components/connect-accounts";
import type { Workflow } from "@/lib/types";

/**
 * Launch dialog: one field per text value in the workflow's stored default input
 * (the plan's {{input.x}} values: topic, recipient email, Telegram chat id, …),
 * pre-filled, editable for this one run. Values the plan left empty are ones only
 * the user knows, so launch waits until they are filled. Per-run values override
 * the workflow defaults server-side.
 */
export function RunWorkflowDialog({
  open,
  onOpenChange,
  workflow,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workflow: Workflow;
}) {
  const router = useRouter();
  const defaults = (workflow.defaultInput ?? {}) as Record<string, unknown>;
  const keys = React.useMemo(
    () => Object.keys(defaults).filter((k) => typeof defaults[k] === "string"),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [workflow.defaultInput],
  );
  const [values, setValues] = React.useState<Record<string, string>>({});
  const [launching, setLaunching] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  // null while loading or when the check itself failed (launch then decides).
  const [preflight, setPreflight] = React.useState<RunPreflight | null>(null);

  React.useEffect(() => {
    if (open) {
      setValues(Object.fromEntries(keys.map((k) => [k, String(defaults[k])])));
      setError(null);
      setLaunching(false);
      setPreflight(null);
      fetchPreflight(workflow.slug).then(setPreflight).catch(() => setPreflight(null));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const blocked = preflight !== null && !preflight.ready;
  const missing = keys.filter((k) => !(values[k] ?? "").trim());

  const onLaunch = async (skipCredentialCheck = false) => {
    setLaunching(true);
    setError(null);
    try {
      const input: Record<string, unknown> = {};
      for (const k of keys) {
        const v = (values[k] ?? "").trim();
        if (v) input[k] = v;
      }
      const runId = await launchRun(workflow.slug, input, { skipCredentialCheck });
      onOpenChange(false);
      router.push(`/runs/${runId}`);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to launch run");
      setLaunching(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogHeader
        title={`Run ${workflow.name}`}
        subtitle="Tweak the inputs for this run, then launch. You’ll watch it live."
        onClose={() => onOpenChange(false)}
      />
      <div className="space-y-3 px-5 py-4">
        {keys.map((k) => (
          <Field key={k} label={inputLabel(k)}>
            <Input
              type={/email/i.test(k) ? "email" : "text"}
              value={values[k] ?? ""}
              onChange={(e) => setValues((prev) => ({ ...prev, [k]: e.target.value }))}
              placeholder={inputHint(k)}
            />
          </Field>
        ))}
        {missing.length > 0 && (
          <p className="text-[12px] text-muted">
            Fill in {missing.map(inputLabel).join(", ")} to launch.
          </p>
        )}
        {preflight && !preflight.ready && (
          <ConnectAccounts slug={workflow.slug} preflight={preflight} onChange={setPreflight} />
        )}
        {error && (
          <div className="rounded-md border border-danger/30 bg-danger-bg px-3 py-2 text-[12px] text-danger">
            {error}
          </div>
        )}
      </div>
      <div className="flex items-center justify-end gap-2 border-t border-border px-5 py-3">
        {blocked && (
          <button
            type="button"
            className="mr-auto text-[12px] text-faint underline-offset-2 hover:underline"
            title="Launch anyway, e.g. when the server's environment already provides these tokens"
            onClick={() => onLaunch(true)}
            disabled={launching || missing.length > 0}
          >
            Run anyway
          </button>
        )}
        <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant="primary" size="sm" onClick={() => onLaunch()} disabled={launching || blocked || missing.length > 0}>
          {launching ? <Loader2 className="size-4 animate-spin" /> : <Play className="size-4" />}
          Launch run
        </Button>
      </div>
    </Dialog>
  );
}

/** "chat_id" → "Chat ID", "email" → "Email the digest to". */
function inputLabel(key: string): string {
  if (key === "email") return "Email the digest to";
  if (key === "topic") return "Topic";
  return key
    .replace(/[_-]+/g, " ")
    .replace(/\bid\b/gi, "ID")
    .replace(/^\w/, (c) => c.toUpperCase());
}

function inputHint(key: string): string | undefined {
  if (key === "topic") return "AI";
  if (/email/i.test(key)) return "you@gmail.com";
  if (/chat_?id/i.test(key)) return "Numeric chat id, e.g. 123456789";
  return undefined;
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1 block text-[12px] font-medium text-muted">{label}</span>
      {children}
    </label>
  );
}
