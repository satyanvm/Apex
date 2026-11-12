"use client";

import * as React from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { LayoutDashboard, LogOut } from "lucide-react";
import { fetchMe, signInHref, signOut, type AccountUser } from "@/lib/auth";
import { cn } from "@/lib/utils";

/** Top-right account area: Sign in / Sign up when signed out, an account menu
 *  when signed in. */
export function AccountNav({ className }: { className?: string }) {
  const pathname = usePathname() ?? "/";
  const [user, setUser] = React.useState<AccountUser | null | undefined>(undefined);

  React.useEffect(() => {
    let active = true;
    fetchMe()
      .then((u) => active && setUser(u))
      .catch(() => active && setUser(null));
    return () => {
      active = false;
    };
  }, []);

  if (user === undefined) {
    // Reserve the space so the header doesn't jump when the answer arrives.
    return <div className={cn("h-8 w-[136px]", className)} aria-hidden />;
  }

  if (!user) {
    const back = pathname === "/" || pathname.startsWith("/sign") ? "/dashboard" : pathname;
    return (
      <div className={cn("flex items-center gap-1.5", className)}>
        <Link
          href={signInHref(back, "signin")}
          className="flex h-8 items-center rounded-md px-3 text-[13px] font-medium text-fg transition-colors hover:bg-surface-2"
        >
          Sign in
        </Link>
        <Link
          href={signInHref(back, "signup")}
          className="flex h-8 items-center rounded-md bg-accent px-3 text-[13px] font-medium text-accent-fg transition-colors hover:bg-accent-soft"
        >
          Sign up
        </Link>
      </div>
    );
  }

  return <AccountMenu user={user} inConsole={!["/", "/playground"].includes(pathname)} className={className} />;
}

function AccountMenu({ user, inConsole, className }: { user: AccountUser; inConsole: boolean; className?: string }) {
  const [open, setOpen] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const ref = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    if (!open) return;
    const close = (e: MouseEvent | KeyboardEvent) => {
      if (e instanceof KeyboardEvent ? e.key === "Escape" : !ref.current?.contains(e.target as Node)) setOpen(false);
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);

  const onSignOut = async () => {
    setBusy(true);
    try {
      await signOut();
    } finally {
      window.location.assign("/");
    }
  };

  return (
    <div ref={ref} className={cn("relative", className)}>
      <button
        onClick={() => setOpen((v) => !v)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`Account: ${user.email}`}
        className="flex h-8 w-8 items-center justify-center rounded-full bg-accent text-[12px] font-semibold text-accent-fg transition-opacity hover:opacity-90"
      >
        {user.initials}
      </button>
      {open && (
        <div
          role="menu"
          className="absolute right-0 top-10 z-50 w-60 overflow-hidden rounded-lg border border-border bg-surface shadow-pop"
        >
          <div className="border-b border-border px-3 py-2.5">
            <p className="truncate text-[13px] font-medium text-fg">{user.name}</p>
            <p className="truncate text-[12px] text-muted">{user.email}</p>
          </div>
          {!inConsole && (
            <Link
              href="/dashboard"
              role="menuitem"
              className="flex items-center gap-2 px-3 py-2 text-[13px] text-fg transition-colors hover:bg-surface-2"
            >
              <LayoutDashboard className="size-4 text-muted" /> Open console
            </Link>
          )}
          <button
            role="menuitem"
            onClick={onSignOut}
            disabled={busy}
            className="flex w-full items-center gap-2 px-3 py-2 text-left text-[13px] text-fg transition-colors hover:bg-surface-2 disabled:opacity-60"
          >
            <LogOut className="size-4 text-muted" /> {busy ? "Signing out…" : "Sign out"}
          </button>
        </div>
      )}
    </div>
  );
}
