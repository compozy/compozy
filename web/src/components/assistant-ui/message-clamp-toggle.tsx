import { Button, cn } from "@compozy/ui";

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
    <Button
      type="button"
      variant="link"
      size="xs"
      data-testid={testId}
      aria-expanded={expanded}
      onClick={onToggle}
      className={cn(
        "h-auto w-fit px-1 text-transcript-caption font-normal text-subtle hover:text-fg aria-expanded:text-subtle aria-expanded:hover:text-fg",
        className
      )}
    >
      {expanded ? "Show less" : "Show more"}
    </Button>
  );
}
