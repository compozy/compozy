import { cn } from "@compozy/ui";

import type { WorkspacesStripOverflow } from "../hooks/use-workspaces-strip-scroll";

interface StripEdgeProps {
  side: "start" | "end";
  visible: boolean;
}

/** Glass dissolve + dark veil marking hidden tiles on an overflowing edge. */
function StripEdge({ side, visible }: StripEdgeProps) {
  return (
    <span
      aria-hidden="true"
      data-slot="os-workspaces-strip-edge"
      data-side={side}
      className={cn(
        "pointer-events-none absolute inset-y-0 z-2 w-14 opacity-0 transition-opacity duration-slow ease-out min-[960px]:w-[72px]",
        side === "start"
          ? "left-0 rounded-l-xl bg-(image:--shell-edge-fade-start)"
          : "right-0 rounded-r-xl bg-(image:--shell-edge-fade-end)",
        visible && "opacity-100"
      )}
    />
  );
}

export interface OsWorkspacesStripProps {
  overflow: WorkspacesStripOverflow;
  dragging: boolean;
  reducedMotion: boolean;
  trackRef: React.RefObject<HTMLDivElement | null>;
  trackProps: React.ComponentProps<"div">;
  /** Tile buttons; the row is the `listbox`, tiles are its options. */
  children: React.ReactNode;
}

/**
 * The one flourish: a frosted Command-Tab panel holding a single scrolling
 * row of tiles. Hugs its content until overflow, then fills the stage and the
 * track scrolls with hidden scrollbars and proximity snap — never a wrap.
 */
export function OsWorkspacesStrip({
  overflow,
  dragging,
  reducedMotion,
  trackRef,
  trackProps,
  children,
}: OsWorkspacesStripProps) {
  const overflowing = overflow !== "none";
  return (
    <div
      data-slot="os-workspaces-strip"
      data-testid="os-workspaces-strip"
      data-overflow={overflow}
      className={cn(
        "relative isolate flex max-w-full flex-nowrap items-stretch overflow-hidden",
        "rounded-xl border border-line-strong bg-shell-glass-pop",
        "shadow-shell-strip backdrop-blur-shell-strip backdrop-saturate-[1.25]",
        overflowing ? "w-full" : "w-max"
      )}
    >
      <StripEdge side="start" visible={overflow === "start" || overflow === "start end"} />
      <StripEdge side="end" visible={overflow === "end" || overflow === "start end"} />
      <div
        ref={trackRef}
        data-slot="os-workspaces-track"
        className={cn(
          "flex max-w-full min-w-0 flex-1 flex-nowrap items-start overflow-x-auto overflow-y-hidden",
          "px-5 pt-4 pb-3.5",
          "overscroll-x-contain [-webkit-overflow-scrolling:touch] [scrollbar-width:none]",
          "touch-pan-x [&::-webkit-scrollbar]:hidden",
          !reducedMotion && !dragging && "snap-x snap-proximity scroll-smooth",
          overflowing && (dragging ? "cursor-grabbing" : "cursor-grab")
        )}
        {...trackProps}
      >
        <div
          role="listbox"
          aria-label="Workspaces"
          aria-orientation="horizontal"
          data-slot="os-workspaces-row"
          className="flex w-max flex-nowrap items-start gap-[5px]"
        >
          {children}
        </div>
      </div>
    </div>
  );
}
