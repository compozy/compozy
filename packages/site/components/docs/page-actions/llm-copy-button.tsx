"use client";

import { Button } from "@agh/ui";
import { Check, Copy } from "lucide-react";
import { useEffect, useRef, useState } from "react";

const cache = new Map<string, string>();
const RESET_DELAY_MS = 1500;

export interface LLMCopyButtonProps {
  markdownUrl: string;
}

export function LLMCopyButton({ markdownUrl }: LLMCopyButtonProps) {
  const [copyPending, setCopyPending] = useState(false);
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const pendingRef = useRef(false);
  const resetTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimeoutRef.current) {
        window.clearTimeout(resetTimeoutRef.current);
      }
    };
  }, []);

  const scheduleReset = (state: "copied" | "failed") => {
    if (resetTimeoutRef.current) {
      window.clearTimeout(resetTimeoutRef.current);
    }
    setCopyState(state);
    resetTimeoutRef.current = window.setTimeout(() => {
      setCopyState("idle");
      resetTimeoutRef.current = null;
    }, RESET_DELAY_MS);
  };

  const onClick = async () => {
    if (pendingRef.current) {
      return;
    }

    pendingRef.current = true;
    setCopyPending(true);
    try {
      const cached = cache.get(markdownUrl);
      if (cached !== undefined) {
        await navigator.clipboard.writeText(cached);
        scheduleReset("copied");
        return;
      }

      const response = await fetch(markdownUrl);
      if (!response.ok) {
        scheduleReset("failed");
        return;
      }

      const content = await response.text();
      cache.set(markdownUrl, content);
      await navigator.clipboard.writeText(content);
      scheduleReset("copied");
    } catch {
      scheduleReset("failed");
    } finally {
      pendingRef.current = false;
      setCopyPending(false);
    }
  };

  const copied = copyState === "copied";
  const label = copyState === "failed" ? "Retry copy" : copied ? "Copied" : "Copy as Markdown";

  return (
    <Button disabled={copyPending} size="sm" variant="outline" onClick={onClick}>
      {copied ? <Check aria-hidden /> : <Copy aria-hidden />}
      {label}
    </Button>
  );
}
