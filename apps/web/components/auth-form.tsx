"use client";

import * as React from "react";
import Link from "next/link";
import { Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { fetchAuthProviders, fetchMe, googleSignInHref, safeNext, signIn, signUp } from "@/lib/auth";

/** The sign-in and sign-up form. After success it does a full navigation to
 *  ?next= so the server sees the new session cookie. */
export function AuthForm({ mode }: { mode: "signin" | "signup" }) {
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [next, setNext] = React.useState("/dashboard");
  const [google, setGoogle] = React.useState(false);

  // Read ?next= from window, not useSearchParams, to avoid the static-render
  // Suspense requirement on this page. Someone already signed in goes straight
  // on, so the landing page's "Start building" can point here for everyone.
  React.useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const target = safeNext(params.get("next"));
    setNext(target);
    if (params.get("error") === "google") {
      setError("Google sign-in didn't complete. Please try again.");
    }
    fetchMe()
      .then((u) => u && window.location.replace(target))
      .catch(() => {});
    fetchAuthProviders().then((p) => setGoogle(p.google));
  }, []);

  const signup = mode === "signup";

  const onSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (signup) await signUp(name.trim(), email.trim(), password);
      else await signIn(email.trim(), password);
      window.location.assign(next);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Something went wrong. Please try again.");
      setBusy(false);
    }
  };

  const other = `/${signup ? "signin" : "signup"}?next=${encodeURIComponent(next)}`;

  return (
    <div className="mx-auto w-full max-w-[380px] px-4 py-14 sm:py-20">
      <h1 className="font-display text-[36px] font-semibold leading-[1.05] text-fg">
        {signup ? "Create your account" : "Sign in to Apex"}
      </h1>
      <p className="mt-1.5 text-[13px] text-muted">
        {signup
          ? "Compiling in the playground is free for everyone. An account lets you save workflows and run them."
          : "Sign in to save and run your workflows."}
      </p>

      {google && (
        <>
          <a
            href={googleSignInHref(next)}
            className="mt-7 flex h-10 w-full items-center justify-center gap-2.5 rounded-md border border-border-strong bg-surface text-[14px] font-medium text-fg transition-colors hover:bg-surface-2"
          >
            <GoogleLogo />
            Continue with Google
          </a>
          <div className="mt-6 flex items-center gap-3 text-[12px] text-faint">
            <span className="h-px flex-1 bg-border" />
            or with email
            <span className="h-px flex-1 bg-border" />
          </div>
        </>
      )}

      <form onSubmit={onSubmit} className={google ? "mt-6 space-y-4" : "mt-7 space-y-4"} noValidate>
        {signup && (
          <Field label="Name" htmlFor="name" hint="Optional">
            <Input id="name" autoComplete="name" value={name} onChange={(e) => setName(e.target.value)} maxLength={80} />
          </Field>
        )}
        <Field label="Email" htmlFor="email">
          <Input
            id="email"
            type="email"
            autoComplete="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </Field>
        <Field label="Password" htmlFor="password" hint={signup ? "At least 8 characters" : undefined}>
          <Input
            id="password"
            type="password"
            autoComplete={signup ? "new-password" : "current-password"}
            required
            minLength={signup ? 8 : undefined}
            maxLength={128}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
        </Field>

        {error && (
          <p role="alert" className="rounded-md bg-danger-bg px-3 py-2 text-[13px] text-danger">
            {error}
          </p>
        )}

        <button
          type="submit"
          disabled={busy || !email.trim() || !password}
          className="flex h-10 w-full items-center justify-center gap-2 rounded-md bg-accent text-[14px] font-medium text-accent-fg transition-colors hover:bg-accent-soft disabled:cursor-not-allowed disabled:opacity-60"
        >
          {busy && <Loader2 className="size-4 animate-spin" />}
          {signup ? "Create account" : "Sign in"}
        </button>
      </form>

      <p className="mt-6 text-center text-[13px] text-muted">
        {signup ? "Already have an account? " : "New to Apex? "}
        <Link href={other} className="font-medium text-accent hover:underline">
          {signup ? "Sign in" : "Create an account"}
        </Link>
      </p>
    </div>
  );
}

function Field({
  label,
  htmlFor,
  hint,
  children,
}: {
  label: string;
  htmlFor: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="mb-1.5 flex items-baseline justify-between">
        <label htmlFor={htmlFor} className="text-[13px] font-medium text-fg">
          {label}
        </label>
        {hint && <span className="text-[12px] text-faint">{hint}</span>}
      </div>
      {children}
    </div>
  );
}

function GoogleLogo() {
  return (
    <svg viewBox="0 0 48 48" className="size-[18px]" aria-hidden>
      <path fill="#FFC107" d="M43.6 20.1H42V20H24v8h11.3C33.7 32.7 29.2 36 24 36c-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.6-.4-3.9z" />
      <path fill="#FF3D00" d="m6.3 14.7 6.6 4.8C14.7 15.1 19 12 24 12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 16.3 4 9.7 8.3 6.3 14.7z" />
      <path fill="#4CAF50" d="M24 44c5.2 0 9.9-2 13.4-5.2l-6.2-5.2C29.2 35.1 26.7 36 24 36c-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.5 39.6 16.2 44 24 44z" />
      <path fill="#1976D2" d="M43.6 20.1H42V20H24v8h11.3c-.8 2.2-2.2 4.2-4.1 5.6l6.2 5.2C37 39.2 44 34 44 24c0-1.3-.1-2.6-.4-3.9z" />
    </svg>
  );
}
