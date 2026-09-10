import { useId } from "react";
import { ChevronDown } from "lucide-react";
import {
  CopyIconButton,
  Popover,
  PopoverContent,
  PopoverTitle,
  PopoverTrigger,
  cn,
} from "@compozy/ui";

/** Compact chrome with a keyboard/pointer detail surface that does not move the transcript. */
export function SessionSummaryDisclosure({
  summary,
  detail,
  label,
  className,
}: {
  summary: string;
  detail: string;
  label: string;
  className?: string;
}) {
  const summaryId = useId();
  return (
    <Popover>
      <PopoverTrigger
        aria-label={label}
        aria-describedby={summaryId}
        className={cn(
          "inline-flex min-w-0 max-w-sm items-center gap-1 rounded-sm text-left hover:bg-hover focus-visible:shadow-focus-inset focus-visible:outline-none",
          className
        )}
      >
        <span id={summaryId} className="min-w-0 truncate">
          {summary}
        </span>
        <ChevronDown aria-hidden="true" className="size-3 shrink-0 text-faint" />
      </PopoverTrigger>
      <PopoverContent align="start" side="top" className="w-sm max-w-[calc(100vw-2rem)]">
        <div className="flex items-center justify-between gap-2">
          <PopoverTitle>{label}</PopoverTitle>
          <CopyIconButton value={detail} copyLabel={`Copy ${label.toLowerCase()}`} />
        </div>
        <pre
          tabIndex={0}
          className="max-h-64 overflow-auto whitespace-pre-wrap break-words font-mono text-transcript-body select-text [overflow-wrap:anywhere]"
        >
          {detail}
        </pre>
      </PopoverContent>
    </Popover>
  );
}
