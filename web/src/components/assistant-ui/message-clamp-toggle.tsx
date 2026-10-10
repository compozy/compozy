import { cn } from "@/lib/utils";

/** The quiet "Show more" / "Show less" under a clamped body; absent when nothing is clamped. */
export function MessageClampToggle({
  clampable,
  expanded,
  onToggle,
  className,
  testId,
}: {
  clampable: boolean;
  expanded: boolean;
  onToggle: () => void;
  className?: string;
  testId: string;
}) {
  if (!clampable) return null;
  return (
    <button
      type="button"
      data-testid={testId}
      aria-expanded={expanded}
      onClick={onToggle}
      className={cn(
        "rounded-xs px-1 text-transcript-caption text-subtle transition-colors duration-base ease-out hover:text-fg",
        className
      )}
    >
      {expanded ? "Show less" : "Show more"}
    </button>
  );
}
