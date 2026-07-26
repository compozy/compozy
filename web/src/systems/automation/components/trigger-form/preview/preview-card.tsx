import type { ReactNode } from "react";
import type { LucideIcon } from "lucide-react";

import { cn, Eyebrow } from "@agh/ui";

interface PreviewCardProps {
  label: ReactNode;
  icon?: LucideIcon;
  right?: ReactNode;
  className?: string;
  children: ReactNode;
}

/** Shared shell for a live-preview card: eyebrow label row + optional right slot + body. */
export function PreviewCard({ label, icon: Icon, right, className, children }: PreviewCardProps) {
  return (
    <div className={cn("rounded-lg border border-line-soft bg-canvas-soft p-3.5", className)}>
      <div className="mb-2.5 flex items-center justify-between gap-2">
        <Eyebrow className="flex items-center gap-1.5 text-faint">
          {Icon ? <Icon aria-hidden="true" className="size-3" /> : null}
          {label}
        </Eyebrow>
        {right}
      </div>
      {children}
    </div>
  );
}
