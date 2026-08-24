import { useEffect, useEffectEvent, useRef } from "react";
import { shallowEqual } from "@xstate/store";

import { notifyUser } from "@/lib/user-feedback";
import {
  toggleSessionSidebar,
  useSessionSidebarState,
  visibleSessionOrder,
  type SessionListViewModel,
  type SessionPayload,
} from "@/systems/session";
import { useProfileLens, useSwitchProfile } from "@/systems/profiles";
import { useWorktreeListings } from "@/systems/workspace";

import { frameSeamEdits } from "../lib/frame-seams";
import { desktopShellBridge, type DesktopShellEventMap } from "../lib/desktop-shell-bridge";
import type { OsAttentionSections, OsSessionAttentionRow } from "../lib/attention-model";
import type { PaletteShellHandlers } from "../lib/cmd-palette-client-ops";
import type { ClientCommandChannel } from "../lib/client-command-channel";
import type { OsDesktopRuntimeStore, OsOpenTarget } from "../lib/os-types";
import { shortcutActionLabel } from "../lib/window-manager-shortcuts";
import type {
  ProjectedFrameSeam,
  ProjectedSeam,
  WindowManagerRegisteredClientView,
} from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";
import type { DesktopOverviewSegmentRequest } from "../stores/window-manager-store-types";
import { useDesktop } from "./use-desktop";
import { useDesktopManagerSurfaces } from "./use-desktop-manager-surfaces";
import { useDesktopOverlays } from "./use-desktop-overlays";
import { useDesktopShellState } from "./use-desktop-shell-state";
import type { DesktopShellModel } from "./use-desktop-shell-model";
import { useDesktopTransitionIntent } from "./use-window-manager-store";
import { useAttentionNotifier } from "./use-attention-notifier";
import { useAttentionJump } from "./use-attention-jump";
import { useCmdPaletteDispatch } from "./use-cmd-palette-dispatch";
import { useDocumentTitleBadge } from "./use-document-title-badge";
import { useFocusedSessionId } from "./use-focused-session-id";
import { useOsAttention } from "./use-os-attention";
import { openPaletteView } from "./use-os-palette-view-stack";
import { useOsReducedMotion } from "./use-os-reduced-motion";
import { useOsShell } from "./use-os-shell";
import { useOsShortcuts } from "./use-os-shortcuts";
import { useOsWinLayer } from "./use-os-win-layer";
import { usePaletteRegistry } from "./use-palette-registry";

function subscribeDesktopSummon(
  listener: (payload: DesktopShellEventMap["shell:summon"]) => void
): (() => void) | undefined {
  const bridge = desktopShellBridge();
  return bridge?.on("shell:summon", listener);
}

export interface DesktopShellBodyOptions {
  /** First-run setup owns the shell; the chrome is inert and shortcuts are off. */
  firstRun?: boolean;
  onNewSession: () => void;
  sessionListView: SessionListViewModel;
  /** The daemon's client-command channel reads the current shell seam through this port. */
  clientCommandChannel: ClientCommandChannel;
  /** The attached shell identity the palette invoke header must prove. */
  client: WindowManagerRegisteredClientView | null;
}

/** Threads the live registered client into the one dispatch invoke. */
export function paletteInvokeIdentity(
  client: Pick<WindowManagerRegisteredClientView, "clientId" | "attachmentToken"> | null
): { clientId?: string; attachmentToken?: string } {
  const clientId = client?.clientId.trim() ?? "";
  const attachmentToken = client?.attachmentToken.trim() ?? "";
  return {
    ...(clientId === "" ? {} : { clientId }),
    ...(attachmentToken === "" ? {} : { attachmentToken }),
  };
}

/**
 * Palette `window.tab.new`: dismiss the command surface, open the tab against
 * the focused frame, then publish the returned window as the destination picker.
 */
export async function completePaletteNewTabOpen(options: {
  closePalette: () => void;
  stackTargetWindowId: string | null;
  userOpen: (target: OsOpenTarget) => Promise<string | null>;
}): Promise<void> {
  options.closePalette();
  const windowId = await options.userOpen({
    app: "new-tab",
    ...(options.stackTargetWindowId ? { stackTargetWindowId: options.stackTargetWindowId } : {}),
  });
  if (windowId !== null) {
    windowManagerStore.trigger.paletteIntentRequested({
      intent: { kind: "destination", windowId },
    });
  }
}

function setSeamPreview(seam: ProjectedSeam, deltaPx: number): void {
  windowManagerStore.trigger.seamPreviewSet({
    preview: {
      kind: "split",
      splitId: seam.splitId,
      boundaryIndex: seam.boundaryIndex,
      deltaPx,
    },
  });
}

function setFrameSeamPreview(seam: ProjectedFrameSeam, deltaPx: number): void {
  windowManagerStore.trigger.seamPreviewSet({
    preview: { kind: "frame", seamId: seam.id, deltaPx },
  });
}

function clearSeamPreview(): void {
  windowManagerStore.trigger.seamPreviewCleared();
}

function completeDesktopTransition(): void {
  windowManagerStore.trigger.transitionIntentChanged({ intent: null });
}

function setDesktopManagerOpen(open: boolean): void {
  if (open) {
    windowManagerStore.trigger.overlayOpened({ overlay: { kind: "desktops-overview" } });
    return;
  }
  windowManagerStore.trigger.overlayClosed();
}

function openDesktopOverview(request: DesktopOverviewSegmentRequest): void {
  windowManagerStore.trigger.overviewSegmentRequested({ request });
}

export function adjacentShortcutItem<T extends { id: string }>(
  items: readonly T[],
  currentId: string | null,
  direction: "previous" | "next"
): T | null {
  if (items.length < 2) return null;
  const currentIndex = items.findIndex(item => item.id === currentId);
  const start = currentIndex < 0 ? 0 : currentIndex;
  const offset = direction === "next" ? 1 : -1;
  return items[(start + offset + items.length) % items.length] ?? null;
}

export function shortcutAttentionTarget(
  sections: OsAttentionSections
): OsSessionAttentionRow | null {
  for (const row of [...sections.needsYou, ...sections.finished]) {
    if (row.kind === "session") return row;
  }
  return null;
}

function focusedSessionId(state: OsDesktopRuntimeStore) {
  const focusedId = state.focusedId;
  if (focusedId === null) return null;
  const window = state.windows[focusedId];
  return window?.app === "session" ? window.instanceKey : null;
}

/** Composes the live runtime models consumed by the presentational desktop shell body. */
export function useDesktopShellBody(model: DesktopShellModel, options: DesktopShellBodyOptions) {
  const firstRun = options.firstRun ?? false;
  const desktopRef = useRef<HTMLDivElement>(null);
  const desktop = useDesktopShellState();
  const overlays = useDesktopOverlays();
  const worktreesByWorkspace = useWorktreeListings(model.workspaces, {
    enabled: overlays.activeOverlay === "workspace-menu" || overlays.activeOverlay === "workspaces",
  });
  const attention = useOsAttention(
    model.runtimeWorkspace,
    model.sessionCatalogStreamStatus,
    options.sessionListView.archived
  );
  const jumpToSession = useAttentionJump();
  useDocumentTitleBadge(attention.notificationCount);
  useAttentionNotifier({
    sections: attention.sections,
    streamLive: model.sessionCatalogStreamStatus === "live",
    focusedSessionId: useFocusedSessionId(),
    onOpenBell: () => overlays.setOverlayOpen("bell", true),
  });
  const managerSurfaces = useDesktopManagerSurfaces();
  const winLayer = useOsWinLayer();
  const reducedMotion = useOsReducedMotion();
  const transition = useDesktopTransitionIntent();
  const { manager, coordinator } = useOsShell();
  const paletteRegistry = usePaletteRegistry();
  const profileLens = useProfileLens();
  const switchProfile = useSwitchProfile(profileLens);
  const { collapsedThreadIds } = useSessionSidebarState();
  const shortcutLabels = useDesktop(state => {
    const effective = state.windowManagerConfig?.effectiveShortcuts;
    const platform = typeof navigator === "undefined" ? "" : navigator.platform;
    return {
      palette: shortcutActionLabel(effective, "palette.open", platform),
      workspacePicker: shortcutActionLabel(effective, "workspace.picker", platform),
      globalScope: shortcutActionLabel(effective, "scope.global.toggle", platform),
    };
  }, shallowEqual);
  const pager = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      desktops: state.desktops,
      compact: state.presentation === "compact",
      canSwitchDesktop:
        state.client !== null &&
        state.hydration === "live" &&
        state.connectionStatus === "connected",
    }),
    shallowEqual
  );

  // The shell's half of the dispatch seam: what a `client_op` is allowed to
  // reach in this client. The seam owns which operation runs; this owns what it
  // can touch.
  const paletteShell: PaletteShellHandlers = {
    openPalette: () => overlays.toggleOverlay("palette"),
    openPaletteView: viewId => {
      // Opening resets the stack to the root, so the push has to follow it.
      overlays.setOverlayOpen("palette", true);
      openPaletteView(viewId);
    },
    openCheatsheet: () => overlays.setOverlayOpen("shortcuts", true),
    openDesktops: () => overlays.toggleOverlay("desktops"),
    openWorkspaces: () => overlays.toggleOverlay("workspaces"),
    openNewSession: options.onNewSession,
    toggleSessions: () => overlays.toggleOverlay("sessions"),
    toggleSidebar: toggleSessionSidebar,
    toggleGlobalScope: model.toggleGlobalScope,
    useProfile: profile => {
      if (profile !== "") switchProfile.mutate({ kind: "profile", profile });
    },
    cycleWorkspace: direction => {
      const target = adjacentShortcutItem(model.workspaces, model.activeWorkspaceId, direction);
      if (target) model.setActiveWorkspaceId(target.id);
    },
    cycleSession: direction => {
      const state = manager.getState();
      const visibleSessions = visibleSessionOrder(attention.sessions, {
        scope: options.sessionListView.scope,
        collapsedThreadIds: new Set(collapsedThreadIds),
        collapsedWorkspaceIds: options.sessionListView.collapsedWorkspaceIds,
        workspaceGroups: options.sessionListView.workspaceGroups,
      });
      const target = adjacentShortcutItem<SessionPayload>(
        visibleSessions,
        focusedSessionId(state),
        direction
      );
      if (target) {
        jumpToSession({
          sessionId: target.id,
          agentName: target.agent_name,
          workspaceId: target.workspace_id ?? model.runtimeWorkspaceId ?? "",
        });
      } else {
        notifyUser({ message: "No other visible session.", tone: "info" });
      }
    },
    focusAttention: () => {
      const target = shortcutAttentionTarget(attention.sections);
      if (target) {
        jumpToSession({
          sessionId: target.id,
          agentName: target.agentName,
          workspaceId: target.workspaceId,
        });
      } else {
        notifyUser({ message: "No session needs attention.", tone: "info" });
      }
    },
    openNewTab: stackTargetWindowId => {
      void completePaletteNewTabOpen({
        closePalette: () => overlays.setOverlayOpen("palette", false),
        stackTargetWindowId,
        userOpen: target => coordinator.userOpen(target),
      });
    },
    activateWindow: windowId => void coordinator.userFocus(windowId),
    // The seam already recorded which step it needs; raising the overlay is all
    // that is left, and it must not toggle a palette that is already open.
    openPaletteExecution: () => overlays.setOverlayOpen("palette", true),
  };
  const paletteDispatch = useCmdPaletteDispatch({
    registry: paletteRegistry,
    workspaceId: model.runtimeWorkspaceId,
    shell: paletteShell,
    openApp: (app, route) => void coordinator.userOpen({ app, ...(route ? { route } : {}) }),
    ...paletteInvokeIdentity(options.client),
  });
  const onDesktopSummon = useEffectEvent((payload: DesktopShellEventMap["shell:summon"]) => {
    const dialog = document.querySelector<HTMLElement>('[data-slot="dialog-content"]');
    if (dialog !== null) {
      dialog.focus();
      return;
    }
    overlays.setOverlayOpen("palette", true);
    if (payload.command_id !== "palette.summon.global") {
      void paletteDispatch.runById(payload.command_id);
    }
  });
  useEffect(() => subscribeDesktopSummon(onDesktopSummon), []);
  // The daemon forwards agent-initiated client operations over the same
  // channel; they enter the same table as ⌘W and the palette row (SI-17).
  const executeClientOp = useEffectEvent(paletteDispatch.executeClientOp);
  useEffect(
    () => options.clientCommandChannel.connect((op, payload) => executeClientOp(op, payload)),
    [options.clientCommandChannel]
  );

  useOsShortcuts(paletteRegistry, commandId => void paletteDispatch.runById(commandId), {
    enabled: !firstRun,
    onEscape: () => {
      if (overlays.activeOverlay !== null) return;
      if (document.querySelector('[data-slot="dialog-content"]')) return;
      desktopRef.current?.focus();
    },
  });

  const onResize = (splitId: string, boundaryIndex: number, delta: number) => {
    // Clear the live seam preview only after the resize reconciles, so panes go
    // straight from previewed to committed sizes.
    void manager.resizeLayout(splitId, boundaryIndex, delta).completion.finally(clearSeamPreview);
  };
  const onFrameResize = (seam: ProjectedFrameSeam, deltaPx: number) => {
    const snapshot = manager.getState().snapshot;
    const groups = snapshot?.desktops.find(entry => entry.id === seam.desktopId)?.groups;
    const edits = groups === undefined ? [] : frameSeamEdits(groups, seam, deltaPx);
    if (edits.length === 0) {
      clearSeamPreview();
      return;
    }
    void manager.resizeGroupFrames(seam.desktopId, edits).completion.finally(clearSeamPreview);
  };
  return {
    attention,
    desktop,
    desktopRef,
    manager,
    managerSurfaces,
    onResize,
    onFrameResize,
    onDesktopManagerOpenChange: setDesktopManagerOpen,
    onOpenDesktopOverview: openDesktopOverview,
    onSeamPreview: setSeamPreview,
    onFrameSeamPreview: setFrameSeamPreview,
    onSeamPreviewEnd: clearSeamPreview,
    onTransitionComplete: completeDesktopTransition,
    overlays,
    pager,
    paletteDispatch,
    reducedMotion,
    shortcutLabels,
    transition,
    winLayer,
    worktreesByWorkspace,
  };
}
