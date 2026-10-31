import { useState } from "react";
import { Archive, Bookmark, type LucideIcon } from "@compozy/ui/icons";

import { Icon, Toggle, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";

import { SESSION_ARCHIVED_COPY, SESSION_LATER_COPY } from "../../lib/session-toolbar-copy";

interface ToolbarToggleCopy {
  controlName: string;
  tooltipOn: string;
  tooltipOff: string;
  liveOn: string;
  liveOff: string;
}

interface ToolbarToggleProps {
  copy: ToolbarToggleCopy;
  icon: LucideIcon;
  on: boolean;
  busy: boolean;
  onChange: (on: boolean) => void;
  slot: string;
  testId: string;
  /** A quiet population count beside the icon; nothing renders at zero or while unknown. */
  count?: number;
}

/**
 * One icon button in the list toolbar that always means one mode.
 *
 * Busy is `aria-disabled` rather than `disabled` so the control keeps focus and
 * its tooltip while a write is in flight.
 */
function ToolbarToggle({
  copy,
  icon,
  on,
  busy,
  onChange,
  slot,
  testId,
  count,
}: ToolbarToggleProps) {
  const shownCount = count !== undefined && count > 0 ? (count > 99 ? "99+" : String(count)) : null;
  const [prevOn, setPrevOn] = useState(on);
  const [announcement, setAnnouncement] = useState("");
  if (on !== prevOn) {
    setPrevOn(on);
    setAnnouncement(on ? copy.liveOn : copy.liveOff);
  }

  const handlePressedChange = (next: boolean) => {
    if (busy) return;
    onChange(next);
  };

  return (
    <span className={cn("inline-flex shrink-0", busy && "cursor-not-allowed")} data-slot={slot}>
      <Tooltip>
        <TooltipTrigger
          render={
            <Toggle
              aria-disabled={busy || undefined}
              aria-label={shownCount ? `${copy.controlName}, ${shownCount}` : copy.controlName}
              className={cn(
                "size-7 min-w-7 p-0 text-muted-foreground hover:bg-surface-2 hover:text-fg",
                shownCount && "w-auto gap-1 px-1.5",
                // Pressed is a quiet surface-2 plate with fg ink — never a ring, never accent.
                "aria-pressed:bg-surface-2 aria-pressed:text-fg",
                "data-[state=on]:bg-surface-2 data-[state=on]:text-fg",
                "aria-disabled:hover:bg-transparent aria-disabled:hover:text-muted-foreground",
                "aria-disabled:aria-pressed:text-fg",
                busy && "opacity-50"
              )}
              data-testid={testId}
              onPressedChange={handlePressedChange}
              pressed={on}
            >
              <Icon as={icon} size="lg" />
              {shownCount ? (
                <span className="text-micro tabular-nums" data-testid={`${testId}-count`}>
                  {shownCount}
                </span>
              ) : null}
            </Toggle>
          }
        />
        <TooltipContent align="start" side="bottom">
          {on ? copy.tooltipOn : copy.tooltipOff}
        </TooltipContent>
      </Tooltip>
      <span aria-live="polite" className="sr-only">
        {announcement}
      </span>
    </span>
  );
}

export interface SessionArchivedToggleProps {
  archived: boolean;
  onArchivedChange: (archived: boolean) => void;
  testId: string;
}

/** Session-list contents: the active catalog, or the archive. */
export function SessionArchivedToggle({
  archived,
  onArchivedChange,
  testId,
}: SessionArchivedToggleProps) {
  return (
    <ToolbarToggle
      copy={SESSION_ARCHIVED_COPY}
      icon={Archive}
      on={archived}
      busy={false}
      onChange={onArchivedChange}
      slot="session-archived-toggle"
      testId={testId}
    />
  );
}

export interface SessionLaterToggleProps {
  later: boolean;
  onLaterChange: (later: boolean) => void;
  /** Sessions parked in Later across the list's scope; `undefined` until the count loads. */
  count?: number;
  testId: string;
}

/** Session-list contents: the active catalog, or the sessions parked in Later. */
export function SessionLaterToggle({
  later,
  onLaterChange,
  count,
  testId,
}: SessionLaterToggleProps) {
  return (
    <ToolbarToggle
      copy={SESSION_LATER_COPY}
      count={count}
      icon={Bookmark}
      on={later}
      busy={false}
      onChange={onLaterChange}
      slot="session-later-toggle"
      testId={testId}
    />
  );
}
