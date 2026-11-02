import { useRef, useState, type ReactNode } from "react";
import { Bot } from "@compozy/ui/icons";
import { useReducedMotionConfig } from "motion/react";

import {
  Button,
  InputGroupButton,
  Popover,
  PopoverContent,
  PopoverTrigger,
  StatusDot,
  cn,
} from "@compozy/ui";

import {
  subagentComposerSignal,
  type SubagentComposerSignal,
  type SubagentCounts,
} from "./subagent-format";
import { SubagentPanel } from "./subagent-panel";
import type { SubagentRosterProps } from "./subagent-roster";
import { SubagentRosterSkeleton } from "./subagent-roster-skeleton";
import type { SubagentView } from "./types";

export interface SubagentComposerControlProps {
  subagents: readonly SubagentView[];
  /** What the trigger reports: the rows' counts, or the session's summary before they load. */
  counts: SubagentCounts;
  /**
   * The roster has not answered yet: the popover holds the rows' place (sized by the session's
   * summary) instead of claiming nothing was delegated.
   */
  loading?: boolean;
  /** The rows could not be read; `onRetry` reads them again. */
  failed?: boolean;
  onRetry?: () => void;
  onOpen?: SubagentRosterProps["onOpen"];
  onStop?: SubagentRosterProps["onStop"];
  /** The popover opened or closed; the owner reads the roster while it is open. */
  onOpenChange?: (open: boolean) => void;
  /** The opened subagent's activity; the host owns its transcript. */
  renderContent?: (subagent: SubagentView) => ReactNode;
  /** Hold the live pulse still (rows the stream has not confirmed). */
  stale?: boolean;
  /** Prototypes and stories start from one of the panel's states. */
  defaultOpen?: boolean;
  defaultSelectedId?: string;
}

/** The subagents icon, carrying the avatar's live dot while any subagent runs. */
function SubagentComposerGlyph({
  signal,
  stale,
}: {
  signal: SubagentComposerSignal;
  stale: boolean;
}) {
  const reduced = useReducedMotionConfig();
  const dot = signal.dot;
  return (
    <span className="relative inline-flex">
      <Bot aria-hidden="true" className="size-4" />
      {dot ? (
        <StatusDot
          tone={dot.tone ?? "faint"}
          size="sm"
          className={cn(
            "absolute -right-0.5 -bottom-0.5 ring-2 ring-card group-hover/button:ring-surface-2 group-data-[popup-open]/button:ring-surface-2",
            dot.pulse && !reduced && !stale && "animate-pulse"
          )}
        />
      ) : null}
    </span>
  );
}

function SubagentComposerFailure({ onRetry }: { onRetry?: () => void }) {
  return (
    <div className="flex items-center justify-between gap-2 px-0.5">
      <p className="text-transcript-caption text-muted-foreground">Couldn't load subagents.</p>
      {onRetry ? (
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

/** No rows: a placeholder while they load, or the empty state. */
function SubagentComposerPlaceholder({ loading, count }: { loading: boolean; count: number }) {
  // The summary sizes the placeholder; with none, a couple of rows hold the place.
  if (loading) return <SubagentRosterSkeleton count={count > 0 ? count : 2} />;
  return (
    <div className="flex flex-col gap-0.5 px-0.5 pb-0.5">
      <p className="text-small-body text-fg">Nothing delegated yet.</p>
      <p className="text-transcript-caption text-pretty text-muted-foreground">
        When the agent hands work to a subagent, it shows up here.
      </p>
    </div>
  );
}

/**
 * The composer's subagents control (issue #766): a quiet icon while nothing
 * runs; while subagents work, the icon takes their avatar's live dot and the
 * live count. Its popover hosts the inspector's roster: current rows first,
 * then previous.
 */
export function SubagentComposerControl({
  subagents,
  counts,
  loading = false,
  failed = false,
  onRetry,
  onOpen,
  onStop,
  onOpenChange,
  renderContent,
  stale = false,
  defaultOpen = false,
  defaultSelectedId,
}: SubagentComposerControlProps) {
  const [open, setOpen] = useState(defaultOpen);
  const [selectedId, setSelectedId] = useState<string | null>(defaultSelectedId ?? null);
  const trigger = useRef<HTMLButtonElement>(null);
  const openingDetail = useRef(false);
  const signal = subagentComposerSignal(counts, failed);
  const live = signal.count !== null;
  const hasRows = subagents.length > 0;
  const close = () => {
    setOpen(false);
    onOpenChange?.(false);
  };

  return (
    <Popover
      open={open}
      onOpenChange={(next, details) => {
        // Escape steps back a level first; it closes only from the list.
        const detailOpen = subagents.some(subagent => subagent.id === selectedId);
        if (!next && detailOpen && details.reason === "escape-key") {
          setSelectedId(null);
          return;
        }
        if (next) openingDetail.current = false;
        else setSelectedId(null);
        setOpen(next);
        onOpenChange?.(next);
      }}
    >
      <PopoverTrigger
        render={
          <InputGroupButton
            ref={trigger}
            type="button"
            // Icon-only while idle; the live count widens it into the tool pill.
            size={live ? "tool" : "tool-icon"}
            aria-label={signal.ariaLabel}
            data-testid="composer-subagents-button"
            data-state={signal.state}
            className="shrink-0 data-[popup-open]:bg-surface-2 data-[popup-open]:text-fg"
          />
        }
      >
        <SubagentComposerGlyph signal={signal} stale={stale} />
        {live ? <span className="font-mono text-mono-id tabular-nums">{signal.count}</span> : null}
      </PopoverTrigger>
      <PopoverContent
        finalFocus={() => (openingDetail.current ? false : trigger.current)}
        side="top"
        align="start"
        className="w-[27rem] max-w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0"
      >
        <SubagentPanel
          counts={counts}
          onOpen={
            onOpen
              ? (subagent, options) => {
                  openingDetail.current = true;
                  setSelectedId(null);
                  close();
                  trigger.current?.focus();
                  onOpen(subagent, options);
                }
              : undefined
          }
          onSelect={setSelectedId}
          onStop={onStop}
          placeholder={
            hasRows && !failed ? null : failed ? (
              <SubagentComposerFailure onRetry={onRetry} />
            ) : (
              <SubagentComposerPlaceholder loading={loading} count={counts.total} />
            )
          }
          renderContent={renderContent}
          selectedId={selectedId}
          stale={stale}
          subagents={subagents}
        />
      </PopoverContent>
    </Popover>
  );
}
