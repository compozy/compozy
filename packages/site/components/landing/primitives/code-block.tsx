"use client";

import { useEffect, useRef, useState } from "react";
import { AlertTriangle, Check, Copy } from "lucide-react";
import { Button, Eyebrow } from "@agh/ui";
import { cn } from "@agh/ui/lib/utils";

interface LandingCodeBlockProps {
  code: string;
  language?: string;
  copyable?: boolean;
  caption?: string;
  className?: string;
  /** Prefix each line with a shell prompt. */
  shell?: boolean;
}

export function LandingCodeBlock({
  code,
  language,
  copyable = true,
  caption,
  className,
  shell = false,
}: LandingCodeBlockProps) {
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimerRef.current) {
        clearTimeout(resetTimerRef.current);
      }
    };
  }, []);

  function scheduleReset() {
    if (resetTimerRef.current) {
      clearTimeout(resetTimerRef.current);
    }
    resetTimerRef.current = setTimeout(() => setCopyState("idle"), 1500);
  }

  async function handleCopy() {
    try {
      await navigator.clipboard.writeText(code);
      setCopyState("copied");
      scheduleReset();
    } catch {
      setCopyState("failed");
      scheduleReset();
    }
  }

  const lines = code.split("\n");
  const displayLines = lines.map((line, index) => ({
    id: `${index}:${line}`,
    line,
  }));

  return (
    <div
      className={cn(
        "min-w-0 max-w-full overflow-hidden rounded-diagram border border-line bg-rail",
        className
      )}
    >
      {(caption || language || copyable) && (
        <div className="flex min-w-0 items-start justify-between gap-3 border-b border-line px-4 py-2.5">
          <Eyebrow className="min-w-0 leading-relaxed text-subtle wrap-anywhere">
            {caption ?? language ?? "shell"}
          </Eyebrow>
          {copyable ? (
            <Button
              variant="ghost"
              size="icon-xs"
              onClick={handleCopy}
              aria-label={
                copyState === "copied"
                  ? "Copied"
                  : copyState === "failed"
                    ? "Copy failed"
                    : "Copy to clipboard"
              }
              aria-live="polite"
              className={cn(
                "text-subtle hover:text-accent",
                copyState === "failed" && "text-danger hover:text-danger"
              )}
            >
              {copyState === "copied" ? (
                <Check aria-hidden className="size-3" />
              ) : copyState === "failed" ? (
                <AlertTriangle aria-hidden className="size-3" />
              ) : (
                <Copy aria-hidden className="size-3" />
              )}
            </Button>
          ) : null}
        </div>
      )}
      <pre className="overflow-x-auto p-4 font-mono text-small-body leading-7 text-fg">
        <code>
          {displayLines.map(({ id, line }) => (
            <div key={id} className={line === "" ? "h-[1.1em]" : undefined}>
              {shell && line !== "" && !line.startsWith("#") ? (
                <span className="select-none text-accent">$ </span>
              ) : null}
              {line.startsWith("#") ? <span className="text-subtle">{line}</span> : line}
            </div>
          ))}
        </code>
      </pre>
    </div>
  );
}
