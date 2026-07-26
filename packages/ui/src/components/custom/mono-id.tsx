"use client";

import { CheckIcon, CopyIcon } from "lucide-react";
import * as React from "react";

import { cn } from "../../lib/utils";

export type MonoIdSize = "default" | "sm";

export interface MonoIdProps extends Omit<React.ComponentProps<"span">, "children"> {
  /**
   * Identifier value rendered bare (no Pill chrome). Lowercased by default; set
   * `preserveCase` for case-sensitive identifiers (e.g. `vault:` references).
   */
  value: string;
  /** Renders an inline 14 × 14 copy button next to the identifier. */
  copy?: boolean;
  /** `default` = 10.5 px (`--text-mono-id`); `sm` = 10 px. */
  size?: MonoIdSize;
  /** Accessible label for the copy button when `copy` is true. */
  copyLabel?: string;
  /** Accessible label for the copy button when copied. */
  copiedLabel?: string;
  /**
   * Preserve the value's casing in both the rendered text and the copied
   * string. Default `false` lowercases (generic IDs); set `true` for
   * case-sensitive identifiers such as `vault:` references, whose bytes must
   * round-trip exactly.
   */
  preserveCase?: boolean;
}

const COPY_FEEDBACK_MS = 1200;

const SIZE_CLASSNAME: Record<MonoIdSize, string> = {
  default: "text-mono-id",
  sm: "text-badge",
};

function MonoId({
  value,
  copy = false,
  size = "default",
  copyLabel = "Copy identifier",
  copiedLabel = "Copied",
  preserveCase = false,
  className,
  ...props
}: MonoIdProps) {
  const text = preserveCase ? value : value.toLowerCase();
  return (
    <span
      data-slot="mono-id"
      data-size={size}
      className={cn(
        "inline-flex min-w-0 items-center gap-1 font-mono tracking-mono-id text-faint tabular-nums",
        SIZE_CLASSNAME[size],
        className
      )}
      {...props}
    >
      <span data-slot="mono-id-value" className="truncate">
        {text}
      </span>
      {copy ? (
        <MonoIdCopyButton value={text} copyLabel={copyLabel} copiedLabel={copiedLabel} />
      ) : null}
    </span>
  );
}

function MonoIdCopyButton({
  value,
  copyLabel,
  copiedLabel,
}: {
  value: string;
  copyLabel: string;
  copiedLabel: string;
}) {
  const [copied, setCopied] = React.useState(false);
  const timerRef = React.useRef<ReturnType<typeof setTimeout> | null>(null);

  React.useEffect(
    () => () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    },
    []
  );

  const handleCopy = async () => {
    if (typeof navigator === "undefined" || !navigator.clipboard?.writeText) return;
    try {
      await navigator.clipboard.writeText(value);
      setCopied(true);
      if (timerRef.current) clearTimeout(timerRef.current);
      timerRef.current = setTimeout(() => setCopied(false), COPY_FEEDBACK_MS);
    } catch {
      // Clipboard may be unavailable in insecure contexts; swallow silently.
    }
  };

  return (
    <button
      type="button"
      data-slot="mono-id-copy"
      data-copied={copied ? "true" : undefined}
      aria-label={copied ? copiedLabel : copyLabel}
      onClick={event => {
        event.preventDefault();
        void handleCopy();
      }}
      className="inline-flex size-3 shrink-0 items-center justify-center rounded-xs text-subtle transition-colors duration-base ease-out hover:bg-hover hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
    >
      {copied ? (
        <CheckIcon width={10} height={10} strokeWidth={2} />
      ) : (
        <CopyIcon width={10} height={10} strokeWidth={1.75} />
      )}
    </button>
  );
}

export { MonoId };
