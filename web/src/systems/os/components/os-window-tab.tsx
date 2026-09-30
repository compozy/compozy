import { Pill, Tooltip, TooltipContent, TooltipTrigger, type TopbarSlotStore } from "@compozy/ui";
import { X } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

import { useWindowMemberSlot } from "../hooks/use-window-member-slot";
import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import { sessionTabState, type OsWindowTabState } from "./os-window-tab-state";
import {
  getSessionDisplayTitle,
  pendingInteractions,
  type SessionPayload,
} from "@/systems/session";

const TAB_STATE_LABELS: Record<Exclude<OsWindowTabState, null>, string> = {
  running: "Session running",
  "needs-input": "Session needs input",
  attention: "Session needs attention",
  quiet: "Session idle",
};

function TabStateDot({ state, label }: { state: OsWindowTabState; label: string }) {
  if (state === null) return null;
  return (
    <Pill.Dot
      aria-hidden={undefined}
      className="shrink-0"
      pulse={state === "running"}
      size="sm"
      tone={state === "running" ? "success" : state === "quiet" ? "neutral" : "accent"}
      data-state={state}
      role="img"
      aria-label={label}
    />
  );
}

export interface OsWindowTabProps {
  win: OsWindow;
  active: boolean;
  session?: SessionPayload | undefined;
  slotStore?: TopbarSlotStore | undefined;
  onActivate: () => void;
  onClose: () => void;
  onCloseOthers: () => void;
  onTabPointerDown?: (event: React.PointerEvent<HTMLElement>) => void;
}

/**
 * One browser tab (prototype `.tab`): glyph or state dot, the live leaf label,
 * the pending-decision count, and a close control shown on hover or when
 * active; pinned tabs are glyph-only. Inactive tabs are quiet text on the
 * sunken strip; the active tab is a canvas plate with rounded top corners and
 * concave feet that fuses with the head below. Selection stays neutral —
 * accent appears only for state or attention (BR-14). The deck's tab slot owns
 * width; the tab only fills it.
 */
export function OsWindowTab({
  win,
  active,
  session,
  slotStore,
  onActivate,
  onClose,
  onCloseOthers,
  onTabPointerDown,
}: OsWindowTabProps) {
  const app = getOsAppDescriptor(win.app);
  const slot = useWindowMemberSlot(slotStore);
  const isSession = win.app === "session";
  const isNewTab = win.app === "new-tab";
  const sessionTitle = isSession && session ? getSessionDisplayTitle(session) : null;
  const label: React.ReactNode = sessionTitle ?? slot?.crumb ?? app.title;
  const showLabel = !win.pinned;
  const pendingCount = isSession && session ? pendingInteractions(session).length : 0;

  const closeTab = (event: React.MouseEvent) => {
    event.stopPropagation();
    if (event.altKey) onCloseOthers();
    else onClose();
  };

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div
            data-slot="os-window-tab"
            data-window-id={win.id}
            data-active={active ? "" : undefined}
            data-pinned={win.pinned ? "" : undefined}
            data-testid={`os-window-tab-${win.id}`}
            className={cn(
              "group/tab relative inline-flex h-deck-tab min-w-0 items-center gap-2 rounded-t-deck-tab text-small-body font-medium transition-colors duration-base select-none",
              win.pinned ? "shrink-0 pr-3" : "flex-1 pr-1.75",
              active
                ? // The plate's side and top hairlines are inset so the feet meet them flush.
                  "z-1 bg-canvas text-fg shadow-[inset_1px_0_0_var(--color-line),inset_-1px_0_0_var(--color-line),inset_0_1px_0_var(--color-line)]"
                : "text-muted hover:bg-surface-2 hover:text-fg"
            )}
          >
            {active ? <OsWindowTabFeet /> : null}
            <button
              type="button"
              role="tab"
              aria-selected={active}
              data-slot="os-window-tab-activate"
              className="flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 self-stretch rounded-[inherit] pl-3 text-left focus-visible:outline-none focus-visible:shadow-focus-inset"
              onPointerDown={event => {
                if (event.button === 0) onTabPointerDown?.(event);
              }}
              onClick={onActivate}
              onAuxClick={event => {
                if (event.button === 1 && !win.pinned) {
                  event.preventDefault();
                  onClose();
                }
              }}
            >
              <OsWindowTabGlyph
                app={app}
                isNewTab={isNewTab}
                isSession={isSession}
                session={session}
              />
              {showLabel ? (
                <span className={cn("min-w-0 flex-1 truncate", isNewTab && "text-subtle")}>
                  {label}
                </span>
              ) : null}
              <Pill.Count count={pendingCount} data-slot="os-window-tab-count" />
            </button>
            {showLabel ? (
              <OsWindowTabClose active={active} label={label} onClick={closeTab} />
            ) : null}
          </div>
        }
      />
      <TooltipContent side="bottom">
        <span className="flex max-w-64 min-w-0 items-center gap-1">
          {(slot?.crumbs ?? []).map(part => (
            <React.Fragment key={part.id}>
              <span className="truncate">{part.label}</span>
              <span aria-hidden="true">/</span>
            </React.Fragment>
          ))}
          <span className="truncate">{sessionTitle ?? label}</span>
        </span>
      </TooltipContent>
    </Tooltip>
  );
}

/**
 * Leading mark: a session shows its state dot, the new-tab page shows nothing,
 * every other app shows its glyph.
 */
function OsWindowTabGlyph({
  app,
  isNewTab,
  isSession,
  session,
}: {
  app: ReturnType<typeof getOsAppDescriptor>;
  isNewTab: boolean;
  isSession: boolean;
  session: SessionPayload | undefined;
}) {
  if (isSession) {
    const state = sessionTabState(session);
    return <TabStateDot state={state} label={state ? TAB_STATE_LABELS[state] : "Session idle"} />;
  }
  if (isNewTab) return null;
  const AppIcon = app.icon;
  return <AppIcon aria-hidden="true" className="size-3.75 shrink-0" />;
}

/**
 * Concave feet (DESIGN-NOTES round 3): a 10px box per side whose fully
 * rounded bottom corner (clamped to the box, so a 10px radius) draws the arc from the tab's side hairline into the strip's
 * bottom hairline, overlapping the tab edge by 1px; its canvas box-shadow
 * fills the plate side of the arc so there is no seam.
 */
function OsWindowTabFeet() {
  const foot =
    "pointer-events-none absolute bottom-0 size-2.5 border-0 border-b border-solid border-line";
  return (
    <>
      <span
        aria-hidden="true"
        data-slot="os-window-tab-foot"
        className={cn(
          foot,
          "-left-2.25 rounded-br-full border-r shadow-[5px_5px_0_5px_var(--color-canvas)]"
        )}
      />
      <span
        aria-hidden="true"
        data-slot="os-window-tab-foot"
        className={cn(
          foot,
          "-right-2.25 rounded-bl-full border-l shadow-[-5px_5px_0_5px_var(--color-canvas)]"
        )}
      />
    </>
  );
}

function OsWindowTabClose({
  active,
  label,
  onClick,
}: {
  active: boolean;
  label: React.ReactNode;
  onClick: (event: React.MouseEvent) => void;
}) {
  return (
    <button
      type="button"
      aria-label={`Close ${typeof label === "string" ? label : "tab"}`}
      data-slot="os-window-tab-close"
      className={cn(
        "grid size-deck-close shrink-0 place-items-center rounded-pill text-subtle opacity-0 transition-opacity duration-base",
        "hover:bg-selected hover:text-fg focus-visible:opacity-100 focus-visible:shadow-focus-ring focus-visible:outline-none",
        "group-hover/tab:opacity-100",
        active && "opacity-100"
      )}
      onPointerDown={event => event.stopPropagation()}
      onClick={onClick}
    >
      <X aria-hidden="true" className="size-3" />
    </button>
  );
}
