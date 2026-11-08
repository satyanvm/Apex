import Link from "next/link";
import { ThemeToggle } from "@/components/ui/theme-toggle";

export const REPO_URL = "https://github.com/satyanvm/Apex";

export function Wordmark() {
  return (
    <Link href="/" className="flex items-center gap-2 text-[15px] font-semibold tracking-tight text-fg">
      <svg viewBox="0 0 24 24" className="size-5" aria-hidden>
        <circle cx="5" cy="6" r="2.6" fill="var(--color-accent)" />
        <circle cx="5" cy="18" r="2.6" fill="var(--color-accent-soft)" />
        <circle cx="19" cy="12" r="2.6" fill="var(--color-accent)" />
        <path d="M7.5 6.8 16.6 11M7.5 17.2l9.1-4.2" stroke="var(--color-accent-soft)" strokeWidth="1.6" fill="none" />
      </svg>
      Apex
    </Link>
  );
}

export function SiteNav({ cta = true }: { cta?: boolean }) {
  return (
    <header className="border-b border-border bg-bg">
      <div className="flex h-14 items-center gap-6 px-5">
        <Wordmark />
        <nav className="ml-auto flex items-center gap-1 text-[13px] sm:gap-3">
          <a className="px-2 py-1 text-muted transition-colors hover:text-fg" href={REPO_URL} rel="noreferrer">
            GitHub
          </a>
          <ThemeToggle />
          {cta && (
            <Link
              href="/playground"
              className="flex h-8 items-center rounded-md bg-accent px-3 font-medium text-accent-fg transition-colors hover:bg-accent-soft"
            >
              Try the compiler
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
