import type { ComponentProps } from "react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@agh/ui";

interface LoopRunQuietNoteProps extends ComponentProps<"div"> {
  icon: LucideIcon;
}

/**
 * The shared quiet-note chrome (§2/§5.3): a hairline card with a leading glyph
 * and one muted line. Used by the "What happens next" note and the no-op outcome.
 */
export function LoopRunQuietNote({
  icon: Icon,
  className,
  children,
  ...props
}: LoopRunQuietNoteProps) {
  return (
    <div
      className={cn(
        "flex items-start gap-2.25 rounded-md border border-line-soft bg-canvas-soft px-3.5 py-3 text-small-body leading-relaxed text-muted",
        className
      )}
      {...props}
    >
      <Icon aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
      <span>{children}</span>
    </div>
  );
}
