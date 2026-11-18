/** Email + password accounts (POST /api/auth/*). The session is an HttpOnly
 *  cookie set by the API, so nothing here stores a token. */

export interface AccountUser {
  id: string;
  name: string;
  email: string;
  initials: string;
}

/** sessionStorage key for a playground prompt waiting for sign-in. */
export const PENDING_PROMPT_KEY = "apex:pending-prompt";

/** The API's error message from its JSON envelope, or null. */
export function apiErrorMessage(body: string): string | null {
  try {
    const message = JSON.parse(body)?.error?.message;
    return typeof message === "string" && message ? message : null;
  } catch {
    return null;
  }
}

async function post(path: string, body?: unknown): Promise<Response> {
  const res = await fetch(path, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: "no-store",
  });
  if (!res.ok) {
    const text = await res.text();
    throw new Error(apiErrorMessage(text) ?? "Something went wrong. Please try again.");
  }
  return res;
}

export async function fetchMe(): Promise<AccountUser | null> {
  const res = await fetch("/api/auth/me", { cache: "no-store" });
  if (!res.ok) return null;
  const data = (await res.json()) as { user: AccountUser | null };
  return data.user;
}

export async function signIn(email: string, password: string): Promise<AccountUser> {
  const res = await post("/api/auth/signin", { email, password });
  return ((await res.json()) as { user: AccountUser }).user;
}

export async function signUp(name: string, email: string, password: string): Promise<AccountUser> {
  const res = await post("/api/auth/signup", { name, email, password });
  return ((await res.json()) as { user: AccountUser }).user;
}

export async function signOut(): Promise<void> {
  await post("/api/auth/signout");
}

/** A same-site path to return to after signing in; anything else goes to the console. */
export function safeNext(next: string | null | undefined): string {
  if (next && next.startsWith("/") && !next.startsWith("//") && !next.startsWith("/\\")) return next;
  return "/dashboard";
}

/** Where the sign-in page should send someone back to from here. */
export function signInHref(path: string, mode: "signin" | "signup" = "signin"): string {
  return `/${mode}?next=${encodeURIComponent(path)}`;
}

/** Full-page navigation target for "Continue with Google". The API redirects to
 *  Google, then back to `next` with the session cookie set. */
export function googleSignInHref(next: string): string {
  return `/api/auth/google/start?next=${encodeURIComponent(safeNext(next))}`;
}

/** Whether the server has Google sign-in configured. */
export async function fetchAuthProviders(): Promise<{ google: boolean }> {
  try {
    const res = await fetch("/api/auth/providers", { cache: "no-store" });
    if (!res.ok) return { google: false };
    const data = (await res.json()) as { google?: boolean };
    return { google: Boolean(data.google) };
  } catch {
    return { google: false };
  }
}
