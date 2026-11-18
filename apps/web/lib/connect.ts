/**
 * One-click account connections (apps/api/internal/handler/connections.go):
 * OAuth2 sign-in popups, and Telegram deep links that need no pasted tokens.
 */
import { apiErrorMessage } from "@/lib/auth";
import { listCredentials, type CredentialSummary } from "@/lib/credentials";

export const TELEGRAM_TYPES = new Set(["pieces.telegram-bot", "telegram"]);

let oauthTypes: Promise<Set<string>> | null = null;

/** Credential types this server can connect with a sign-in popup. */
export function fetchOAuthTypes(): Promise<Set<string>> {
  if (!oauthTypes) {
    oauthTypes = fetch("/api/oauth/apps", { cache: "no-store" })
      .then((r) => (r.ok ? r.json() : { types: [] }))
      .then((d: { types?: string[] }) => new Set(d.types ?? []))
      .catch(() => new Set<string>());
    // A failed or empty answer is retried next time (the worker may be starting).
    oauthTypes.then((s) => {
      if (s.size === 0) oauthTypes = null;
    });
  }
  return oauthTypes;
}

interface ConnectMessage {
  type: "apex-connect";
  ok: boolean;
  credential?: CredentialSummary;
  error?: string;
}

/**
 * Open the provider's sign-in in a popup and resolve with the saved
 * credential. Resolves null if the user closes the popup first.
 */
export function connectWithPopup(credentialType: string, name?: string): Promise<CredentialSummary | null> {
  const qs = name ? `?name=${encodeURIComponent(name)}` : "";
  const url = `/api/oauth/${encodeURIComponent(credentialType)}/start${qs}`;
  const w = 520;
  const h = 680;
  const left = window.screenX + Math.max(0, (window.outerWidth - w) / 2);
  const top = window.screenY + Math.max(0, (window.outerHeight - h) / 2);
  const popup = window.open(url, "apex-connect", `popup,width=${w},height=${h},left=${left},top=${top}`);
  if (!popup) {
    return Promise.reject(new Error("Your browser blocked the sign-in window. Allow pop-ups for this site and try again."));
  }
  return new Promise((resolve, reject) => {
    let done = false;
    const finish = (fn: () => void) => {
      if (done) return;
      done = true;
      window.removeEventListener("message", onMessage);
      window.clearInterval(timer);
      fn();
    };
    const onMessage = (e: MessageEvent) => {
      if (e.origin !== window.location.origin) return;
      const msg = e.data as ConnectMessage | undefined;
      if (!msg || msg.type !== "apex-connect") return;
      finish(() =>
        msg.ok && msg.credential
          ? resolve(msg.credential)
          : reject(new Error(msg.error || "The connection didn't complete.")),
      );
    };
    window.addEventListener("message", onMessage);
    const timer = window.setInterval(() => {
      if (popup.closed) finish(() => resolve(null));
    }, 500);
  });
}

/* ------------------------------ Telegram ------------------------------ */

export interface TelegramInfo {
  platformBot: boolean;
  botUsername?: string;
}

export interface TelegramStart {
  code: string;
  botUsername: string;
  link: string;
  expiresAt: string;
}

export interface TelegramStatus {
  state: "waiting" | "connected" | "expired" | "failed";
  credentialId?: string;
  chatName?: string;
  error?: string;
}

export interface TelegramChat {
  credentialId: string;
  name: string;
  chatId: string;
  chatName?: string;
}

export async function fetchTelegramInfo(): Promise<TelegramInfo> {
  try {
    const res = await fetch("/api/connect/telegram", { cache: "no-store" });
    return res.ok ? ((await res.json()) as TelegramInfo) : { platformBot: false };
  } catch {
    return { platformBot: false };
  }
}

export async function startTelegram(input: {
  credentialType: string;
  name?: string;
  botToken?: string;
}): Promise<TelegramStart> {
  const res = await fetch("/api/connect/telegram/start", {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify(input),
    cache: "no-store",
  });
  const text = await res.text();
  if (!res.ok) throw new Error(apiErrorMessage(text) ?? "Couldn't start the Telegram connection.");
  return JSON.parse(text) as TelegramStart;
}

export async function telegramStatus(code: string): Promise<TelegramStatus> {
  const res = await fetch(`/api/connect/telegram/status?code=${encodeURIComponent(code)}`, { cache: "no-store" });
  if (!res.ok) return { state: "waiting" };
  return (await res.json()) as TelegramStatus;
}

export async function fetchTelegramChats(): Promise<TelegramChat[]> {
  try {
    const res = await fetch("/api/connect/telegram/chats", { cache: "no-store" });
    if (!res.ok) return [];
    return ((await res.json()) as { items?: TelegramChat[] }).items ?? [];
  } catch {
    return [];
  }
}

/** The saved credential's summary, once a connect flow reports its id. */
export async function credentialById(id: string): Promise<CredentialSummary | null> {
  return (await listCredentials()).find((c) => c.id === id) ?? null;
}
