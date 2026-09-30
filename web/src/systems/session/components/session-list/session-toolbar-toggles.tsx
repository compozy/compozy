import { useState } from "react";
import { Archive, Globe, type LucideIcon } from "lucide-react";

import { Icon, Toggle, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";

import { SESSION_ARCHIVED_COPY, SESSION_SCOPE_COPY } from "../../lib/session-toolbar-copy";

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
}

/**
 * Deliberately the same globe-shaped control as the menubar scope toggle, so
 * one icon button in this row always means one mode.
 *
 * Busy is `aria-disabled` rather than `disabled` so the control keeps focus and
 * its tooltip while a write is in flight.
 */
function ToolbarToggle({ copy, icon, on, busy, onChange, slot, testId }: ToolbarToggleProps) {
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
              aria-label={copy.controlName}
              className={cn(
                "size-7 min-w-7 p-0 text-muted hover:bg-surface-2 hover:text-fg",
                // Pressed reads like an active pill: surface-2 + shadow-card, fg ink — never accent.
                "aria-pressed:bg-surface-2 aria-pressed:text-fg aria-pressed:shadow-card",
                "data-[state=on]:bg-surface-2 data-[state=on]:text-fg data-[state=on]:shadow-card",
                "aria-disabled:hover:bg-transparent aria-disabled:hover:text-muted",
                "aria-disabled:aria-pressed:text-fg",
                busy && "opacity-50"
              )}
              data-testid={testId}
              onPressedChange={handlePressedChange}
              pressed={on}
            >
              <Icon as={icon} size="lg" />
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

export interface SessionScopeToggleProps {
  allWorkspaces: boolean;
  /** True while the daemon has not acknowledged the scope write yet. */
  busy?: boolean;
  onAllWorkspacesChange: (allWorkspaces: boolean) => void;
  testId: string;
}

/** Session-list breadth: this workspace, or every workspace. */
export function SessionScopeToggle({
  allWorkspaces,
  busy = false,
  onAllWorkspacesChange,
  testId,
}: SessionScopeToggleProps) {
  return (
    <ToolbarToggle
      copy={SESSION_SCOPE_COPY}
      icon={Globe}
      on={allWorkspaces}
      busy={busy}
      onChange={onAllWorkspacesChange}
      slot="session-scope-toggle"
      testId={testId}
    />
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
