import { useEffect, useState } from "react";

/** Copies text and reports "copied" for a moment. */
export function useCopied() {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => setCopied(false), 1600);
    return () => window.clearTimeout(timer);
  }, [copied]);
  const copy = (text: string) => {
    void navigator.clipboard?.writeText(text);
    setCopied(true);
  };
  return { copied, copy };
}
