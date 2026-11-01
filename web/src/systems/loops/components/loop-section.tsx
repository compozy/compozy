import type { ComponentProps, ReactNode } from "react";
import { ChevronDown } from "@compozy/ui/icons";

import { cn, Collapsible, CollapsibleContent, CollapsibleTrigger, Eyebrow } from "@compozy/ui";

// The header's frame and icon well live here once, so `LoopSectionSkeleton` and the real section
// stay the same height and the title lands on the same line.
const SECTION_HEADER_CLASS = "flex min-h-6 items-center gap-2 pb-2";
const SECTION_ICON_CLASS = "shrink-0 text-subtle [&_svg]:size-3.5";

interface LoopSectionProps extends Omit<ComponentProps<typeof Collapsible>, "title"> {
  icon: ReactNode;
  title: string;
  /** One-line takeaway kept on the header while the section is folded. */
  gist?: ReactNode;
  right?: ReactNode;
}

/**
 * Collapsible main-column section: Lucide icon + eyebrow title + optional gist
 * + right slot + chevron. The right slot is a sibling of the triggers so it can
 * hold a link or button. Rail cards stay on `LoopRailSection`.
 */
export function LoopSection({
  icon,
  title,
  gist,
  right,
  defaultOpen = true,
  className,
  children,
  ...props
}: LoopSectionProps) {
  return (
    <Collapsible className={cn("group/sec", className)} defaultOpen={defaultOpen} {...props}>
      <div className={SECTION_HEADER_CLASS}>
        <CollapsibleTrigger className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <span className={SECTION_ICON_CLASS}>{icon}</span>
          <Eyebrow className="text-subtle">{title}</Eyebrow>
          {gist ? (
            <span className="min-w-0 shrink truncate text-form-hint text-faint">{gist}</span>
          ) : null}
        </CollapsibleTrigger>
        {right ? <span className="shrink-0">{right}</span> : null}
        <CollapsibleTrigger aria-hidden="true" className="shrink-0 text-faint" tabIndex={-1}>
          <ChevronDown
            aria-hidden="true"
            className={cn(
              "size-3.5 shrink-0 transition-transform -rotate-90",
              "group-data-open/sec:rotate-0 motion-reduce:transition-none"
            )}
          />
        </CollapsibleTrigger>
      </div>
      <CollapsibleContent>{children}</CollapsibleContent>
    </Collapsible>
  );
}

interface LoopSectionSkeletonProps extends Omit<ComponentProps<"div">, "title"> {
  icon: ReactNode;
  title: string;
  /** Placeholder for the header's right slot (a button or link the loaded section carries). */
  right?: ReactNode;
  /** False draws the chevron of a section that starts folded; the body is then not shown. */
  open?: boolean;
}

/**
 * A `LoopSection` while its body is being read. The icon and title are static, so the header is
 * the real one without the fold behaviour; the body is whatever placeholder the caller passes.
 */
export function LoopSectionSkeleton({
  icon,
  title,
  right,
  open = true,
  className,
  children,
  ...props
}: LoopSectionSkeletonProps) {
  return (
    <div className={className} {...props}>
      <div className={SECTION_HEADER_CLASS}>
        <span className="flex min-w-0 flex-1 items-center gap-2 text-left">
          <span className={SECTION_ICON_CLASS}>{icon}</span>
          <Eyebrow className="text-subtle">{title}</Eyebrow>
        </span>
        {right ? <span className="shrink-0">{right}</span> : null}
        <ChevronDown
          aria-hidden="true"
          className={cn("size-3.5 shrink-0 text-faint", !open && "-rotate-90")}
        />
      </div>
      {open ? children : null}
    </div>
  );
}
