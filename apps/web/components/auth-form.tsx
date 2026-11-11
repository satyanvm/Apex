"use client";

import * as React from "react";
import Link from "next/link";
import { Loader2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { safeNext, signIn, signUp } from "@/lib/auth";

/** The sign-in and sign-up form. After success it does a full navigation to
 *  ?next= so the server sees the new session cookie. */
export function AuthForm({ mode }: { mode: "signin" | "signup" }) {
  const [name, setName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [next, setNext] = React.useState("/dashboard");

  // Read ?next= from window, not useSearchParams, to avoid the static-render
  // Suspense requirement on this page.
  React.useEffect(() => {
    setNext(safeNext(new URLSearchParams(window.location.search).get("next")));
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
      <h1 className="text-[22px] font-semibold tracking-tight text-fg">
        {signup ? "Create your account" : "Sign in to Apex"}
      </h1>
      <p className="mt-1.5 text-[13px] text-muted">
        {signup
          ? "Compiling in the playground is free for everyone. An account lets you save workflows and run them."
          : "Sign in to save and run your workflows."}
      </p>

      <form onSubmit={onSubmit} className="mt-7 space-y-4" noValidate>
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
