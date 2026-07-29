import { ArrowDown } from "lucide-react";

import { cn } from "@/lib/utils";

/**
 * Floating scroll-to-bottom affordance: a neutral `size-8` disc (no
 * glass/backdrop-blur) that fades + drifts in with the shared disclosure
 * motion and stays mounted so its exit animates too.
 */
export function ScrollToBottomPill({
  visible,
  onClick,
}: {
  visible: boolean;
  onClick: () => void;
}) {
  return (
    <div
      className={cn(
        "pointer-events-none absolute inset-x-0 bottom-3 z-20 flex justify-center",
        "transition-[transform,opacity] duration-base ease-out motion-reduce:transition-none",
        visible ? "translate-y-0 opacity-100" : "translate-y-1 opacity-0"
      )}
      aria-hidden={!visible}
    >
      <button
        type="button"
        data-testid="scroll-to-bottom-pill"
        data-visible={visible}
        aria-label="Scroll to latest"
        onClick={onClick}
        tabIndex={visible ? 0 : -1}
        className={cn(
          "flex size-8 items-center justify-center rounded-full",
          "border border-line bg-canvas-soft text-muted shadow-[var(--shadow-overlay)]",
          "transition-colors hover:bg-hover hover:text-fg",
          "focus-visible:shadow-focus-ring focus-visible:outline-none",
          visible ? "pointer-events-auto" : "pointer-events-none"
        )}
      >
        <ArrowDown className="size-3.5" aria-hidden="true" />
      </button>
    </div>
  );
}
