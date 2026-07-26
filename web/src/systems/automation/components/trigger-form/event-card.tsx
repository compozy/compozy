import { Brain, Check, CircleStop, Play, Puzzle, Webhook, Workflow } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn } from "@agh/ui";

import type { EventIconKey } from "../../lib/trigger-catalog";

const EVENT_ICONS: Record<EventIconKey, LucideIcon> = {
  "session-start": Play,
  "session-stop": CircleStop,
  memory: Brain,
  hook: Workflow,
  webhook: Webhook,
  extension: Puzzle,
};

interface EventCardProps {
  catalogId: string;
  displayId: string;
  label: string;
  description: string;
  disabled?: boolean;
  icon: EventIconKey;
  selected: boolean;
  onSelect: () => void;
}

/** One selectable runtime event in the catalog. */
export function EventCard({
  catalogId,
  displayId,
  label,
  description,
  disabled = false,
  icon,
  selected,
  onSelect,
}: EventCardProps) {
  const Icon = EVENT_ICONS[icon];
  return (
    <button
      aria-pressed={selected}
      disabled={disabled}
      className={cn(
        "flex w-full items-start gap-3 rounded-md border p-3 text-left transition-colors outline-none focus-visible:shadow-focus-ring",
        disabled
          ? "cursor-not-allowed border-line-soft bg-canvas-tint opacity-50"
          : selected
            ? "border-transparent bg-accent-tint ring-1 ring-accent-dim ring-inset"
            : "border-line-soft bg-canvas-tint hover:border-line hover:bg-elevated"
      )}
      data-testid={`trigger-event-${catalogId}`}
      onClick={onSelect}
      type="button"
    >
      <span
        className={cn(
          "flex size-7 shrink-0 items-center justify-center rounded",
          selected ? "bg-accent-tint-strong text-accent-strong" : "bg-badge-fill text-muted"
        )}
      >
        <Icon aria-hidden="true" className="size-4" />
      </span>
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="flex flex-wrap items-baseline gap-2">
          <span
            className={cn(
              "font-mono text-form-label font-medium",
              selected ? "text-accent-strong" : "text-fg-strong"
            )}
          >
            {displayId}
          </span>
          <span className="text-form-label text-muted">{label}</span>
        </span>
        <span className="text-form-hint leading-snug text-subtle">{description}</span>
      </span>
      <Check
        aria-hidden="true"
        className={cn(
          "mt-0.5 size-4 shrink-0 text-accent-strong transition-opacity",
          selected ? "opacity-100" : "opacity-0"
        )}
      />
    </button>
  );
}
