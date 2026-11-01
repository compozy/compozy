import { StateGlyph, type StateGlyphState, type TopbarSlotStore } from "@compozy/ui";
import * as React from "react";

import { useWindowMemberSlot } from "../hooks/use-window-member-slot";
import { getOsAppDescriptor, osAppId, type OsAppDescriptor } from "../lib/app-catalog";
import type { OsWindow } from "../lib/os-types";
import { sessionTabState, type OsWindowTabState } from "./os-window-tab-state";
import { OsWindowTabView } from "./os-window-tab-view";
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

/** Tab vocabulary → the canonical StateGlyph states (StateGlyph mapping table). */
const TAB_STATE_GLYPH: Record<Exclude<OsWindowTabState, null>, StateGlyphState> = {
  running: "running",
  "needs-input": "attention",
  attention: "attention",
  quiet: "idle",
};

function TabStateGlyph({ state, label }: { state: OsWindowTabState; label: string }) {
  if (state === null) return null;
  return <StateGlyph state={TAB_STATE_GLYPH[state]} size="sm" label={label} />;
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
  const slot = useWindowMemberSlot(slotStore);
  const appId = osAppId(win.app);
  if (appId === null) return null;
  const app = getOsAppDescriptor(appId);
  const isSession = win.app === "session";
  const isNewTab = win.app === "new-tab";
  const sessionTitle = isSession && session ? getSessionDisplayTitle(session) : null;
  const label: React.ReactNode = sessionTitle ?? slot?.crumb ?? app.title;

  const closeTab = (event: React.MouseEvent) => {
    event.stopPropagation();
    if (event.altKey) onCloseOthers();
    else onClose();
  };

  return (
    <OsWindowTabView
      active={active}
      crumbs={slot?.crumbs}
      glyph={
        <OsWindowTabGlyph app={app} isNewTab={isNewTab} isSession={isSession} session={session} />
      }
      isNewTab={isNewTab}
      label={label}
      onActivate={onActivate}
      onClose={closeTab}
      onMiddleClose={onClose}
      onTabPointerDown={onTabPointerDown}
      pendingCount={tabPendingCount(isSession, session)}
      pinned={win.pinned}
      title={sessionTitle ?? label}
      windowId={win.id}
    />
  );
}

function tabPendingCount(isSession: boolean, session: SessionPayload | undefined): number {
  return isSession && session ? pendingInteractions(session).length : 0;
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
  app: OsAppDescriptor;
  isNewTab: boolean;
  isSession: boolean;
  session: SessionPayload | undefined;
}) {
  if (isSession) {
    const state = sessionTabState(session);
    return <TabStateGlyph state={state} label={state ? TAB_STATE_LABELS[state] : "Session idle"} />;
  }
  if (isNewTab) return null;
  const AppIcon = app.icon;
  return (
    <AppIcon aria-hidden="true" className="size-3.5 shrink-0" data-slot="os-window-tab-icon" />
  );
}
