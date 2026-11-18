"use client";

import * as React from "react";
import { CheckCircle2, ExternalLink, Loader2, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { CredentialSummary } from "@/lib/credentials";
import {
  connectWithPopup,
  credentialById,
  fetchTelegramInfo,
  startTelegram,
  telegramStatus,
  type TelegramInfo,
  type TelegramStart,
} from "@/lib/connect";
import type { CredentialTypeDef } from "./credential-types";

/** "Sign in with Gmail": the provider's consent screen in a popup. */
export function OAuthConnect({
  typeDef,
  name,
  onConnected,
}: {
  typeDef: CredentialTypeDef;
  name: string;
  onConnected: (cred: CredentialSummary) => void | Promise<void>;
}) {
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const connect = async () => {
    setBusy(true);
    setError(null);
    try {
      const cred = await connectWithPopup(typeDef.id, name.trim() || undefined);
      if (cred) await onConnected(cred);
      else setBusy(false);
    } catch (e) {
      setError(e instanceof Error ? e.message : "The connection didn't complete.");
      setBusy(false);
    }
  };

  return (
    <div className="space-y-2.5">
      <Button type="button" variant="primary" className="w-full" onClick={connect} disabled={busy}>
        {busy ? <Loader2 className="size-4 animate-spin" /> : <ExternalLink className="size-4" />}
        {busy ? "Waiting for sign-in…" : `Sign in with ${typeDef.label}`}
      </Button>
      <p className="text-[12px] text-muted">
        A window opens where you sign in to {typeDef.label} and approve access. Apex never sees your password, and
        keeps the connection signed in for you.
      </p>
      {error && <p className="break-words text-[12px] text-danger">{error}</p>}
    </div>
  );
}

/**
 * Telegram without copying tokens or chat ids: open a link, tap Start, done.
 * Uses the server's bot when it has one; otherwise (or by choice) the user's
 * own bot, whose token they paste once.
 */
export function TelegramConnect({
  typeDef,
  name,
  onConnected,
}: {
  typeDef: CredentialTypeDef;
  name: string;
  onConnected: (cred: CredentialSummary) => void | Promise<void>;
}) {
  const [info, setInfo] = React.useState<TelegramInfo | null>(null);
  const [ownBot, setOwnBot] = React.useState(false);
  const [token, setToken] = React.useState("");
  const [start, setStart] = React.useState<TelegramStart | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [connected, setConnected] = React.useState<string | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const onConnectedRef = React.useRef(onConnected);
  onConnectedRef.current = onConnected;

  React.useEffect(() => {
    fetchTelegramInfo().then((i) => {
      setInfo(i);
      if (!i.platformBot) setOwnBot(true);
    });
  }, []);

  // Poll until the Start message arrives.
  React.useEffect(() => {
    if (!start) return;
    let stopped = false;
    const tick = async () => {
      if (stopped) return;
      const s = await telegramStatus(start.code).catch(() => null);
      if (stopped) return;
      if (s?.state === "connected" && s.credentialId) {
        setConnected(s.chatName ?? "your chat");
        const cred = await credentialById(s.credentialId).catch(() => null);
        if (cred && !stopped) await onConnectedRef.current(cred);
        return;
      }
      if (s?.state === "failed" || s?.state === "expired") {
        setError(s.error || "The link expired before Telegram replied. Start again.");
        setStart(null);
        return;
      }
      window.setTimeout(tick, 2000);
    };
    const t = window.setTimeout(tick, 1500);
    return () => {
      stopped = true;
      window.clearTimeout(t);
    };
  }, [start]);

  const begin = async () => {
    setBusy(true);
    setError(null);
    // Open the tab inside the click, or the browser blocks it; point it at
    // Telegram once the link exists.
    const tab = window.open("", "_blank");
    try {
      const s = await startTelegram({
        credentialType: typeDef.id,
        name: name.trim() || undefined,
        botToken: ownBot ? token.trim() : undefined,
      });
      if (tab) tab.location.href = s.link;
      setStart(s);
    } catch (e) {
      tab?.close();
      setError(e instanceof Error ? e.message : "Couldn't start the Telegram connection.");
    } finally {
      setBusy(false);
    }
  };

  if (info === null) {
    return <div className="py-2 text-[12px] text-faint">Loading…</div>;
  }

  if (connected) {
    return (
      <div className="flex items-center gap-2 rounded-md border border-success/30 bg-success-bg px-3 py-2.5 text-[13px] text-fg">
        <CheckCircle2 className="size-4 shrink-0 text-success" />
        Connected to {connected}.
      </div>
    );
  }

  if (start) {
    return (
      <div className="space-y-3">
        <div className="rounded-md border border-border bg-surface-2 px-3 py-3">
          <div className="flex items-center gap-2 text-[13px] font-medium text-fg">
            <Loader2 className="size-4 animate-spin text-accent" />
            Waiting for you to tap <span className="font-semibold">Start</span> in Telegram…
          </div>
          <p className="mt-1.5 text-[12px] text-muted">
            Telegram should have opened a chat with @{start.botUsername}. Tap Start there and this window updates by
            itself.
          </p>
        </div>
        <a
          href={start.link}
          target="_blank"
          rel="noreferrer"
          className="inline-flex items-center gap-1.5 text-[12px] font-medium text-accent hover:underline"
        >
          <ExternalLink className="size-3.5" /> Open @{start.botUsername} again
        </a>
        <p className="break-all text-[11px] text-faint">
          On your phone instead? Open {start.link} there.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {!ownBot ? (
        <>
          <Button type="button" variant="primary" className="w-full" onClick={begin} disabled={busy}>
            {busy ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
            Connect Telegram
          </Button>
          <p className="text-[12px] text-muted">
            Opens a chat with @{info.botUsername}. Tap Start, and your workflows can message you there. No token or
            chat id to copy.
          </p>
          <button
            type="button"
            onClick={() => setOwnBot(true)}
            className="text-[12px] text-muted underline-offset-2 hover:text-fg hover:underline"
          >
            Use my own bot instead
          </button>
        </>
      ) : (
        <>
          <ol className="list-decimal space-y-1 pl-4 text-[12px] text-muted">
            <li>
              In Telegram, open{" "}
              <a href="https://t.me/BotFather" target="_blank" rel="noreferrer" className="text-accent hover:underline">
                @BotFather
              </a>{" "}
              and send /newbot.
            </li>
            <li>Paste the token it gives you below.</li>
            <li>Tap Start in the chat that opens. Apex finds the chat id itself.</li>
          </ol>
          <Input
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="123456789:AAH…"
            autoComplete="off"
            spellCheck={false}
            className="font-mono text-[13px]"
          />
          <Button type="button" variant="primary" className="w-full" onClick={begin} disabled={busy || !token.trim()}>
            {busy ? <Loader2 className="size-4 animate-spin" /> : <Send className="size-4" />}
            Continue in Telegram
          </Button>
          {info.platformBot && (
            <button
              type="button"
              onClick={() => setOwnBot(false)}
              className="text-[12px] text-muted underline-offset-2 hover:text-fg hover:underline"
            >
              Use @{info.botUsername} instead
            </button>
          )}
        </>
      )}
      {error && <p className="break-words text-[12px] text-danger">{error}</p>}
    </div>
  );
}
