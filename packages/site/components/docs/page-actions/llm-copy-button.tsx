"use client";

import { Button } from "@compozy/ui";
import { Check, Copy } from "lucide-react";
import { useEffect, useRef, useState } from "react";

const cache = new Map<string, string>();
const RESET_DELAY_MS = 1500;
type CopyResult = "copied" | "failed";

export interface LLMCopyButtonProps {
  markdownUrl: string;
}

async function copyMarkdown(markdownUrl: string): Promise<CopyResult> {
  try {
    const cached = cache.get(markdownUrl);
    if (cached !== undefined) {
      await navigator.clipboard.writeText(cached);
      return "copied";
    }

    const response = await fetch(markdownUrl);
    if (!response.ok) {
      return "failed";
    }

    const content = await response.text();
    cache.set(markdownUrl, content);
    await navigator.clipboard.writeText(content);
    return "copied";
  } catch {
    return "failed";
  }
}

export function LLMCopyButton({ markdownUrl }: LLMCopyButtonProps) {
  const [copyPending, setCopyPending] = useState(false);
  const [copyState, setCopyState] = useState<"idle" | CopyResult>("idle");
  const pendingRef = useRef(false);
  const resetTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    return () => {
      if (resetTimeoutRef.current) {
        window.clearTimeout(resetTimeoutRef.current);
      }
    };
  }, []);

  const scheduleReset = (state: CopyResult) => {
    if (resetTimeoutRef.current) {
      window.clearTimeout(resetTimeoutRef.current);
    }
    setCopyState(state);
    resetTimeoutRef.current = window.setTimeout(() => {
      setCopyState("idle");
      resetTimeoutRef.current = null;
    }, RESET_DELAY_MS);
  };

  const onClick = () => {
    if (pendingRef.current) {
      return;
    }

    pendingRef.current = true;
    setCopyPending(true);
    void copyMarkdown(markdownUrl)
      .then(scheduleReset)
      .finally(() => {
        pendingRef.current = false;
        setCopyPending(false);
      });
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
