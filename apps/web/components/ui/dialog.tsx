"use client";

import * as React from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * A tiny modal primitive modeled on the existing command-palette overlay (same
 * backdrop + click-outside-to-close + stop-propagation), so we add no dependency.
 * Renders nothing when `open` is false; Escape and backdrop click both close.
 *
 * Portaled to <body> so a dialog opened from inside another dialog (e.g. the
 * credential modal from the Run dialog) isn't clipped by the parent's
 * `overflow-hidden` + transform (the `animate-in` keyframes leave a transform,
 * which makes the parent the containing block for `fixed` descendants).
 * Escape only closes the topmost open dialog.
 */
const openStack: symbol[] = [];

export function Dialog({
  open,
  onOpenChange,
  children,
  className,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  children: React.ReactNode;
  className?: string;
}) {
  const [id] = React.useState(() => Symbol("dialog"));
  const onOpenChangeRef = React.useRef(onOpenChange);
  onOpenChangeRef.current = onOpenChange;

  React.useEffect(() => {
    if (!open) return;
    openStack.push(id);
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && openStack[openStack.length - 1] === id) {
        onOpenChangeRef.current(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      const i = openStack.lastIndexOf(id);
      if (i >= 0) openStack.splice(i, 1);
    };
  }, [open, id]);

  if (!open || typeof document === "undefined") return null;

  return createPortal(
    <div
      className="fixed inset-0 z-50 flex items-start justify-center px-4 pt-[10vh]"
      onClick={() => onOpenChange(false)}
    >
      <div className="absolute inset-0 bg-[#0b0e1a]/40 backdrop-blur-[2px] animate-in" />
      <div
        className={cn(
          "relative w-full max-w-[560px] overflow-hidden rounded-xl border border-border-strong bg-surface shadow-pop animate-in",
          className,
        )}
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-modal="true"
      >
        {children}
      </div>
    </div>,
    document.body,
  );
}

export function DialogHeader({
  title,
  subtitle,
  onClose,
}: {
  title: string;
  subtitle?: string;
  onClose: () => void;
}) {
  return (
    <div className="flex items-start justify-between gap-3 border-b border-border px-5 py-4">
      <div className="min-w-0">
        <h2 className="text-[15px] font-semibold tracking-tight text-fg">{title}</h2>
        {subtitle && <p className="mt-0.5 text-[12px] text-muted">{subtitle}</p>}
      </div>
      <button
        type="button"
        onClick={onClose}
        className="flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-faint transition-colors hover:bg-surface-2 hover:text-fg"
        aria-label="Close"
      >
        <X className="size-4" />
      </button>
    </div>
  );
}
