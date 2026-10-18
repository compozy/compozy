import { ChevronRight } from "lucide-react";
import type { ComponentProps, ReactNode } from "react";

import { cn } from "../../lib/utils";

export interface TranscriptDisclosureProps extends Omit<
  ComponentProps<"button">,
  "children" | "onClick" | "onToggle"
> {
  expanded: boolean;
  onToggle: () => void;
  icon: ReactNode;
  label: ReactNode;
  trailing?: ReactNode;
  variant?: "row" | "turn";
}

const CHEVRON_CLASS =
  "size-3 shrink-0 text-subtle transition-[transform,opacity] duration-slow ease-out motion-reduce:transition-none";

/**
 * The transcript's disclosure trigger: a controlled toggle whose body the
 * caller renders (and points at with `aria-controls`). `row` is the tool/work
 * line — the kind glyph rests in the 18px well and hands its place to a
 * chevron on hover, focus, or open, so the line never grows a trailing
 * control; `turn` is the quiet turn-fold sentence with a leading chevron. Use
 * `Disclosure` instead when the fold should own its own panel.
 */
export function TranscriptDisclosure({
  expanded,
  onToggle,
  icon,
  label,
  trailing,
  variant = "row",
  className,
  ...props
}: TranscriptDisclosureProps) {
  const turn = variant === "turn";

  return (
    <button
      {...props}
      type="button"
      aria-expanded={expanded}
      onClick={onToggle}
      className={cn(
        "group/disclosure",
        turn
          ? "-ml-0.5 inline-flex items-center gap-1 rounded-xs px-1 py-px text-transcript-body text-subtle tabular-nums"
          : "inline-flex min-h-transcript-line w-fit items-center gap-transcript-inline-gap rounded-md px-1 text-left text-small-body text-subtle",
        "transition-colors duration-base ease-out",
        turn ? "hover:text-fg" : "hover:text-muted",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        className
      )}
    >
      {turn ? (
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-2.75 shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            expanded ? "rotate-90" : null
          )}
        />
      ) : (
        <span className="relative flex size-transcript-icon-well shrink-0 items-center justify-center">
          <span
            data-slot="transcript-disclosure-icon"
            className={cn(
              "flex items-center justify-center transition-opacity duration-base ease-out motion-reduce:transition-none",
              expanded
                ? "opacity-0"
                : "group-hover/disclosure:opacity-0 group-focus-visible/disclosure:opacity-0"
            )}
          >
            {icon}
          </span>
          <ChevronRight
            aria-hidden="true"
            data-slot="transcript-disclosure-chevron"
            className={cn(
              "absolute",
              CHEVRON_CLASS,
              expanded
                ? "rotate-90 opacity-100"
                : "opacity-0 group-hover/disclosure:opacity-100 group-focus-visible/disclosure:opacity-100"
            )}
          />
        </span>
      )}
      <span className={cn("min-w-0", turn ? null : "shrink truncate")}>{label}</span>
      {trailing}
    </button>
  );
}
