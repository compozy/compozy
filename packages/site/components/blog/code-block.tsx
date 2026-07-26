"use client";

import { cn } from "@agh/ui";
import { AlertTriangle, Check, Copy } from "lucide-react";
import { useEffect, useRef, useState, type ComponentProps } from "react";

export interface BlogCodeBlockProps extends ComponentProps<"pre"> {
  "data-language"?: string;
}

export function BlogCodeBlock({ className, children, ...props }: BlogCodeBlockProps) {
  const ref = useRef<HTMLPreElement>(null);
  const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");

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

  async function onCopy() {
    const text = ref.current?.textContent ?? "";
    if (!text) {
      return;
    }

    try {
      await navigator.clipboard.writeText(text);
      setCopyState("copied");
    } catch {
      setCopyState("failed");
    }
    scheduleReset();
  }

  return (
    <div className="group relative mt-7 overflow-hidden rounded-xl border border-line bg-rail">
      <button
        type="button"
        onClick={onCopy}
        aria-label={
          copyState === "copied" ? "Copied" : copyState === "failed" ? "Copy failed" : "Copy code"
        }
        aria-live="polite"
        className={cn(
          "absolute right-2 top-2 inline-flex size-7 items-center justify-center rounded-md text-subtle opacity-0 transition-opacity hover:text-fg focus-visible:opacity-100 group-hover:opacity-100",
          copyState === "failed" && "text-danger hover:text-danger"
        )}
      >
        {copyState === "copied" ? (
          <Check size={13} aria-hidden />
        ) : copyState === "failed" ? (
          <AlertTriangle size={13} aria-hidden />
        ) : (
          <Copy size={13} aria-hidden />
        )}
      </button>
      <pre
        ref={ref}
        {...props}
        className={cn(
          "overflow-x-auto px-5 py-4 font-mono text-small-body leading-7 text-fg",
          className
        )}
      >
        {children}
      </pre>
    </div>
  );
}
