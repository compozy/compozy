import { Pill, Tooltip, TooltipContent, TooltipTrigger, type TopbarSlotStore } from "@compozy/ui";
import { X } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

import { useWindowMemberSlot } from "../hooks/use-window-member-slot";
import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import { sessionTabState, type OsWindowTabState } from "./os-window-tab-state";
import { getSessionDisplayTitle, type SessionPayload } from "@/systems/session";

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
 * One deck segment (reference §01): glyph or state dot + the live leaf label,
 * hover ×, attention badge, pinned glyph-only form. Selection stays neutral —
 * accent appears only for state or attention (BR-14). The deck's tab slot owns
 * width — every unpinned tab shares one uniform slot and the label truncates;
 * the tab only fills it.
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
  const state: OsWindowTabState = isSession ? sessionTabState(session) : null;
  const needsInput = state === "needs-input";
  const sessionTitle = session ? getSessionDisplayTitle(session) : null;
  const label: React.ReactNode = isSession
    ? (sessionTitle ?? slot?.crumb ?? app.title)
    : (slot?.crumb ?? app.title);
  const crumbPath = slot?.crumbs ?? [];
  const AppIcon = app.icon;

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
              "group/tab relative inline-flex h-deck-tab items-center rounded-t-deck-tab border border-b-0 border-transparent text-small-body font-medium text-subtle transition-colors duration-base select-none",
              win.pinned ? "min-w-0 shrink-0" : "min-w-0 flex-1",
              active
                ? "border-line bg-canvas font-semibold text-fg-strong shadow-[0_1px_0_var(--color-canvas)]"
                : "hover:bg-canvas-soft hover:text-fg",
              "focus-within:shadow-focus-inset"
            )}
          >
            <button
              type="button"
              role="tab"
              aria-selected={active}
              data-slot="os-window-tab-activate"
              className="flex min-w-0 flex-1 cursor-pointer items-center gap-[7px] rounded-[inherit] px-2.5 text-left focus-visible:outline-none focus-visible:shadow-focus-inset"
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
              {isSession ? (
                <TabStateDot
                  state={state}
                  label={state ? TAB_STATE_LABELS[state] : "Session idle"}
                />
              ) : isNewTab ? null : (
                <AppIcon aria-hidden="true" className="size-deck-glyph shrink-0 text-subtle" />
              )}
              {!win.pinned ? (
                <span className={cn("min-w-0 flex-1 truncate", isNewTab && "text-subtle")}>
                  {label}
                </span>
              ) : null}
              {needsInput ? (
                <span
                  data-slot="os-window-tab-badge"
                  className="inline-flex h-3.5 min-w-deck-badge shrink-0 items-center justify-center rounded-full bg-accent px-1 font-mono text-[9px] leading-none font-bold text-accent-ink"
                >
                  1
                </span>
              ) : null}
            </button>
            {!win.pinned ? (
              <button
                type="button"
                aria-label={`Close ${typeof label === "string" ? label : "tab"}`}
                data-slot="os-window-tab-close"
                className={cn(
                  "grid size-4 shrink-0 place-items-center rounded-xs text-faint opacity-0 transition-opacity duration-base",
                  "hover:bg-btn-default-fill hover:text-fg-strong focus-visible:opacity-100 focus-visible:shadow-focus-ring focus-visible:outline-none",
                  "group-hover/tab:opacity-100",
                  active && "opacity-100"
                )}
                onPointerDown={event => event.stopPropagation()}
                onClick={closeTab}
              >
                <X aria-hidden="true" className="size-[9px]" strokeWidth={1.4} />
              </button>
            ) : null}
          </div>
        }
      />
      <TooltipContent side="bottom">
        <span className="flex max-w-64 min-w-0 items-center gap-1">
          {crumbPath.map(part => (
            <React.Fragment key={part.id}>
              <span className="truncate">{part.label}</span>
              <span aria-hidden="true">/</span>
            </React.Fragment>
          ))}
          <span className="truncate">{isSession && session ? sessionTitle : label}</span>
        </span>
      </TooltipContent>
    </Tooltip>
  );
}
