// Paired example: do (accent check) and don't (destructive cross).
import { cn } from "@compozy/ui/utils";
import type { ReactNode } from "react";

interface DoDontProps {
  doContent: ReactNode;
  dontContent: ReactNode;
  doLabel?: string;
  dontLabel?: string;
  className?: string;
}

export function DoDont({
  doContent,
  dontContent,
  doLabel = "Do",
  dontLabel = "Don't",
  className,
}: DoDontProps) {
  return (
    <div className={cn("grid gap-4 md:grid-cols-2", className)}>
      <div className="min-w-0 rounded-card border border-(--b-line) bg-(--b-card) p-6">
        <p className="m-0 mb-3 mono-label text-(--b-accent)">✓ {doLabel}</p>
        {doContent}
      </div>
      <div className="min-w-0 rounded-card border border-dashed border-[color-mix(in_oklab,var(--color-destructive)_55%,transparent)] bg-(--b-card) p-6">
        <p className="m-0 mb-3 mono-label text-destructive">✕ {dontLabel}</p>
        {dontContent}
      </div>
    </div>
  );
}
