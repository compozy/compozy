import { ChevronDown, ChevronRight } from "lucide-react";
import { useRef, type ComponentProps, type ReactNode } from "react";

import { cn } from "@/lib/utils";

export interface TranscriptDisclosureProps extends Omit<
  ComponentProps<"button">,
  "children" | "onClick" | "onToggle"
> {
  expanded: boolean;
  onToggle: (button: HTMLElement | null) => void;
  icon: ReactNode;
  label: ReactNode;
  trailing?: ReactNode;
  variant?: "row" | "turn";
}

/** Shared transcript disclosure trigger; callers own only the revealed body. */
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
  const ref = useRef<HTMLButtonElement | null>(null);
  const turn = variant === "turn";

  return (
    <button
      {...props}
      ref={ref}
      type="button"
      aria-expanded={expanded}
      onClick={() => onToggle(ref.current)}
      className={cn(
        turn
          ? "-ml-0.5 inline-flex items-center gap-1 rounded-xs px-1 py-px text-transcript-body text-subtle tabular-nums"
          : "inline-flex min-h-transcript-line w-fit items-center gap-transcript-inline-gap rounded-md px-1 text-left text-small-body font-medium text-muted",
        "transition-colors duration-base ease-out hover:text-fg",
        turn ? null : "hover:bg-hover",
        "focus-visible:shadow-focus-ring focus-visible:outline-none",
        className
      )}
    >
      {turn ? (
        <ChevronRight
          aria-hidden="true"
          className={cn(
            "size-[11px] shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            expanded ? "rotate-90" : null
          )}
        />
      ) : (
        <span className="flex size-transcript-icon-well shrink-0 items-center justify-center">
          {icon}
        </span>
      )}
      <span className={cn("min-w-0", turn ? null : "shrink truncate")}>{label}</span>
      {trailing}
      {turn ? null : (
        <ChevronDown
          aria-hidden="true"
          className={cn(
            "size-3 shrink-0 text-faint transition-transform duration-slow ease-out motion-reduce:transition-none",
            expanded ? "rotate-180" : null
          )}
          strokeWidth={1.75}
        />
      )}
    </button>
  );
}
