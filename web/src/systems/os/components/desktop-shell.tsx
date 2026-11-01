import { Outlet, useLocation } from "@tanstack/react-router";

import { cn } from "@compozy/ui";

import { CmdPaletteRegistryProvider } from "../contexts/cmd-palette-registry-context";
import { OsShellContext } from "../contexts/os-shell-context";
import { useCmdPaletteRegistry } from "../hooks/use-cmd-palette-registry";
import { WorktreeDialogActionsContext } from "../contexts/worktree-dialog-actions-context";
import { useDesktopChromeController } from "../hooks/use-desktop-chrome-controller";
import { useDesktop } from "../hooks/use-desktop";
import { useDesktopShellBody } from "../hooks/use-desktop-shell-body";
import type { DesktopShellModel } from "../hooks/use-desktop-shell-model";
import { useProfileAutomationEnablement } from "../hooks/use-profile-automation-enablement";
import { useDesktopWorktreeScope, WindowScopeContext } from "../hooks/use-worktree-scope";
import type { WorktreeDialogTargets } from "../hooks/use-worktree-dialog-targets";
import type { ClientCommandChannel } from "../lib/client-command-channel";
import type { WindowManagerRegisteredClientView } from "../lib/window-manager-types";
import { DesktopGate } from "./desktop-gate";
import { DesktopWorktreeDialogs } from "./desktop-worktree-dialogs";
import { DesktopMenubar } from "./desktop-menubar";
import type { MenubarUpdateModel } from "./menubar/menubar-update-indicator";
import { ShellDesktopDock } from "./desktop-dock";
import { DesktopManagerSurfaces } from "./desktop-manager-surfaces";
import { DesktopPagerSurface } from "./desktop-pager-surface";
import { DesktopSessionDialogs, WorkspaceSetupDialogBoundary } from "./desktop-shell-dialogs";
import { ProcessesLeftRunningHost } from "@/systems/session";

import { OsAboutDialog } from "./os-about-dialog";
import { OsAppPreloader } from "./os-app-preloader";
import { OsCommandPalette } from "./os-command-palette";
import { OsShortcutsDialog } from "./os-shortcuts-dialog";
import { OsWorkspacesOverview } from "./os-workspaces-overview";
import { OsWallpaper } from "./os-wallpaper";
import { TerminalCloseDialog } from "./terminal-close-dialog";
import { LiveOsWinLayer } from "./os-win-layer";
import { AgentCreateHostProvider, AgentCreateScreen } from "@/systems/agent";
import { useOnboardingStatus } from "@/systems/onboarding";
import {
  ProfileLifecycleHost,
  ProfileSwitcherSlot,
  WorkspaceProfilesHint,
} from "@/systems/profiles";
import {
  SessionCreateDialogHost,
  SessionCreateProvider,
  useSessionCreateActions,
  useSessionListView,
} from "@/systems/session";
import { useProductUpdate, useReloadOnProductVersionChange } from "@/systems/settings";
import { selectWorktreeForScope, type WorkspaceSetupDefaultsModel } from "@/systems/workspace";

/**
 * The desktop shell replaces the AppShell chrome (ADR-001): onboarding gate,
 * menubar, rail, wallpapered win-layer, ⌘K palette, and the window-manager
 * sync lifecycle. Route matches render through the (invisible) Outlet as
 * sync-controllers; windows render in the layer.
 */
export function DesktopShell() {
  // Same cached query the gate reads — no extra fetch, one truth for first run.
  const onboarding = useOnboardingStatus();
  // Same cached read and apply key the Updates section uses — one truth.
  const update = useProductUpdate();
  useReloadOnProductVersionChange(update.query.data);
  const firstRun = onboarding.data?.completed === false;

  return (
    <DesktopGate>
      <DesktopChrome
        firstRun={firstRun}
        update={{
          state: update.indicator,
          onUpdate: update.product?.targets ? update.apply : undefined,
        }}
      />
    </DesktopGate>
  );
}

/** Mounts the shell context and its lifecycle confirmation before rendering desktop content. */
function DesktopChrome({ firstRun, update }: { firstRun: boolean; update: MenubarUpdateModel }) {
  const controller = useDesktopChromeController();

  // Zero project workspaces boots into All projects with User focused; the
  // shell never waits for a project before rendering.

  return (
    <WorktreeDialogActionsContext.Provider value={controller.worktreeDialogs}>
      <OsShellContext.Provider value={controller.chrome.shell}>
        <TerminalCloseDialog controller={controller.chrome.terminalClose} />
        <DesktopChromeContent
          client={controller.chrome.client}
          firstRun={firstRun}
          model={controller.model}
          clientCommandChannel={controller.chrome.clientCommandChannel}
          update={update}
          worktreeDialogs={controller.worktreeDialogs}
          workspaceSetupDefaults={controller.workspaceSetupDefaults}
        />
      </OsShellContext.Provider>
    </WorktreeDialogActionsContext.Provider>
  );
}

interface DesktopShellBodyProps {
  client: WindowManagerRegisteredClientView | null;
  model: DesktopShellModel;
  firstRun: boolean;
  update: MenubarUpdateModel;
  workspaceSetupDefaults: WorkspaceSetupDefaultsModel;
  worktreeDialogs: WorktreeDialogTargets;
  /** The daemon's client-command channel reads the current shell seam through this port. */
  clientCommandChannel: ClientCommandChannel;
}

function DesktopChromeContent(props: DesktopShellBodyProps) {
  // The focused window's data binding: first in the New session project prefill.
  const focusedBinding = useDesktop(state => {
    const binding = state.focusedId ? state.windows[state.focusedId]?.route.search.workspace : null;
    return typeof binding === "string" ? binding : null;
  });
  // This projection reads the desktop topology, so it must mount below the
  // shell context that owns the window-manager store.
  const paletteRegistry = useCmdPaletteRegistry({
    // The structural catalog belongs to the attached desktop client. All
    // projects changes the command lens, but it must not detach shell commands
    // from the mounted window-manager partition.
    workspaceId: props.model.desktopWorkspaceId,
    client: props.client,
  });

  return (
    <CmdPaletteRegistryProvider registry={paletteRegistry}>
      <SessionCreateProvider store={props.model.sessionCreate.store}>
        <AgentCreateHostProvider
          openDialog={props.model.agentCreate.openDialog}
          openForDuplicate={props.model.agentCreate.openForDuplicate}
        >
          <DesktopShellBody {...props} />
          <SessionCreateDialogHost
            activeWorkspace={props.model.runtimeWorkspace}
            agents={props.model.workspaceAgents}
            projectWorkspaceId={props.model.activeWorkspaceId}
            dataScope={props.model.dataScope}
            focusedBinding={focusedBinding}
            store={props.model.sessionCreate.store}
          />
        </AgentCreateHostProvider>
      </SessionCreateProvider>
    </CmdPaletteRegistryProvider>
  );
}

type DesktopWorktreeScope = ReturnType<typeof useDesktopWorktreeScope>;

function DesktopShellBody(props: DesktopShellBodyProps) {
  const worktreeScope = useDesktopWorktreeScope(props.model.worktreeListing);

  return (
    <WindowScopeContext value={worktreeScope.worktreeScopeId}>
      <DesktopShellScopedBody {...props} {...worktreeScope} />
    </WindowScopeContext>
  );
}

/** Wires both workspace lists to the same profile authority and lifecycle dialog targets. */
function DesktopShellScopedBody({
  client,
  model,
  firstRun,
  update,
  workspaceSetupDefaults,
  worktreeDialogs,
  worktreeScopeId,
  worktreeSelection,
  clientCommandChannel,
}: DesktopShellBodyProps & DesktopWorktreeScope) {
  // A deep link can arrive before its window-open receipt. Keep that transient
  // empty projection from loading a composer the destination does not use.
  const showEmptyDesktop = useLocation({ select: location => location.pathname === "/" });
  const sessionCreate = useSessionCreateActions();
  const setAutomationEnabled = useProfileAutomationEnablement();
  // Scope and order are the operator's, persisted by the daemon; the modal
  // renders them rather than fetching its own.
  const sessionListView = useSessionListView({
    scope: model.dataScope,
    worktreeId: worktreeSelection.activeWorktree?.id,
    enabled: worktreeSelection.resolved,
    pending: !worktreeSelection.resolved,
  });
  const openNewSession = () => {
    sessionCreate.openForAgent("");
  };
  const {
    attention,
    desktop,
    desktopRef,
    manager,
    managerSurfaces,
    onResize,
    onFrameResize,
    onDesktopManagerOpenChange,
    desktopSwitchShortcuts,
    onOpenDesktopOverview,
    onSeamPreview,
    onFrameSeamPreview,
    onSeamPreviewEnd,
    onTransitionComplete,
    overlays,
    pager,
    paletteDispatch,
    reducedMotion,
    shortcutLabels,
    transition,
    workingSessionsByWorkspace,
    worktreesByWorkspace,
  } = useDesktopShellBody(model, {
    firstRun,
    onNewSession: openNewSession,
    sessionListView,
    clientCommandChannel,
    client,
  });
  return (
    <div
      ref={desktopRef}
      id="app-content"
      data-testid="os-desktop"
      data-first-run={firstRun ? "true" : undefined}
      inert={firstRun}
      tabIndex={-1}
      // Topbar across the full width; rail + desk below it; the compact tab bar
      // takes the last row. Each dock presentation claims its own track. The
      // grid paints the chrome, so topbar + rail read as one L-shaped surface
      // around the inset desk panel.
      className="grid min-h-0 flex-1 grid-cols-[auto_minmax(0,1fr)] grid-rows-[auto_minmax(0,1fr)_auto] overflow-hidden bg-rail focus-visible:shadow-focus-inset focus-visible:outline-none"
    >
      <DesktopMenubar
        // Dimmed while setup blocks: readable enough to see what you unlock,
        // never bright enough to read as available.
        className={cn(
          "col-span-full transition-opacity duration-shell-slow motion-reduce:transition-none",
          firstRun && "opacity-68"
        )}
        // The desktop follows the destination: a project's partition, `user`
        // for User, and the focused project's (or `global`) under All projects.
        layoutUnbound={!model.pending && model.desktopWorkspaceId === null}
        workspaces={model.workspaces}
        activeWorkspace={model.activeWorkspace}
        chip={model.chip}
        breadthMode={model.breadthMode}
        focus={model.focus}
        scopePending={model.pending}
        focusWorkspaceName={model.rememberedWorkspace?.name ?? null}
        onToggleAllProjects={model.toggleAllProjects}
        onSelectWorkspace={workspaceId =>
          model.setActiveWorkspaceId(workspaceId, { scopeId: worktreeScopeId })
        }
        onFocusUser={() => model.focusDestination({ kind: "user" })}
        onEnableAllProjects={model.enableAllProjects}
        onAddWorkspace={model.openWorkspaceSetup}
        onRunCommand={commandId => void paletteDispatch.runById(commandId)}
        activeOverlay={overlays.activeOverlay}
        onOverlayOpenChange={overlays.setOverlayOpen}
        attention={attention}
        update={update}
        worktreesByWorkspace={worktreesByWorkspace}
        workingSessionsByWorkspace={workingSessionsByWorkspace}
        userHomeDir={model.userHomeDir}
        worktreeSelection={worktreeSelection}
        onSelectWorktree={(workspaceId, entry) => {
          // A discovered row is the adoption gesture; anything else scopes.
          if (worktreeDialogs.requestAdopt(workspaceId, entry)) return;
          if (entry.worktree) {
            selectWorktreeForScope(worktreeScopeId, workspaceId, entry.worktree.id);
          }
        }}
        onCreateWorktree={model.openWorktreeCreate}
        onResolveMissingWorktree={worktreeDialogs.requestResolveMissing}
        onOpenWorktreeContext={worktreeDialogs.requestContext}
        onRemoveWorktree={worktreeDialogs.requestRemove}
        onRemoveWorktrees={worktreeDialogs.requestRemoveBatch}
        pager={
          <DesktopPagerSurface
            activeDesktopId={pager.activeDesktopId}
            desktops={pager.desktops}
            needsYouDesktopIds={pager.needsYouDesktopIds}
            canSwitchDesktop={pager.canSwitchDesktop}
            onSelectDesktop={desktopId => manager.switchDesktop(desktopId)}
            onOpenOverview={onOpenDesktopOverview}
          />
        }
      />
      <ShellDesktopDock
        dormant={firstRun}
        onNewSession={openNewSession}
        badges={attention.badges}
        contextMenusEnabled={overlays.activeOverlay === null}
        profileSwitcher={
          <ProfileSwitcherSlot
            onOpenSettings={() => void paletteDispatch.runById("settings.profiles")}
          />
        }
        onOpenSettings={() => void paletteDispatch.runById("settings.general")}
      />
      <div
        data-slot="os-desk"
        className={cn(
          // The desk is a panel inset into the chrome: a hairline where it meets
          // the topbar and rail, flush with the window's right and bottom edges.
          // It clips its children so tiled panes follow the rounded corner; the
          // win-layer sits inside the border, so snap geometry never sees it.
          "relative col-start-2 row-start-2 min-h-0 overflow-hidden border-t border-line",
          desktop.presentation !== "compact" && "rounded-tl-lg border-l"
        )}
      >
        <OsWallpaper wallpaper={desktop.wallpaper} />
        {model.activeWorkspaceId !== null ? (
          <WorkspaceProfilesHint
            hints={model.workspaceProfileHints}
            workspaceId={model.activeWorkspaceId}
          />
        ) : null}
        {desktop.windowIds.map(windowId => (
          <OsAppPreloader key={windowId} windowId={windowId} />
        ))}
        <LiveOsWinLayer
          showEmptyDesktop={showEmptyDesktop}
          paletteShortcutLabel={shortcutLabels.palette}
          canStart={model.dataScope !== null}
          reducedMotion={reducedMotion}
          transition={transition}
          onTransitionComplete={onTransitionComplete}
          onResize={onResize}
          onFrameResize={onFrameResize}
          onSeamPreview={onSeamPreview}
          onFrameSeamPreview={onFrameSeamPreview}
          onSeamPreviewEnd={onSeamPreviewEnd}
        />
        <DesktopManagerSurfaces
          model={managerSurfaces}
          needsYouDesktopIds={pager.needsYouDesktopIds}
          switchShortcuts={desktopSwitchShortcuts}
          onCreateDesktop={() => manager.createDesktop()}
          onSwitchDesktop={desktopId => manager.switchDesktop(desktopId)}
          onRenameDesktop={(desktopId, name) => manager.renameDesktop(desktopId, name)}
          onReorderDesktop={(desktopId, order) => manager.reorderDesktop(desktopId, order)}
          onDeleteDesktop={(desktopId, destinationId) =>
            manager.deleteDesktop(desktopId, destinationId)
          }
          onMoveWindow={(windowId, destinationDesktopId) =>
            manager.moveWindowToDesktop(windowId, destinationDesktopId)
          }
          onOpenChange={onDesktopManagerOpenChange}
          onRetry={() => manager.refreshSnapshot()}
          onResolveConflict={() => {
            manager.clearConflict();
            manager.refreshSnapshot();
          }}
        />
      </div>
      {/* Route matches mount here as sync-controllers; they render null. */}
      <Outlet />
      <OsCommandPalette
        client={client}
        open={overlays.activeOverlay === "palette"}
        onOpenChange={open => overlays.setOverlayOpen("palette", open)}
        dispatch={paletteDispatch}
      />
      <DesktopSessionDialogs
        workspaceId={model.runtimeWorkspaceId}
        open={overlays.activeOverlay === "sessions"}
        onOpenChange={open => overlays.setOverlayOpen("sessions", open)}
        disconnected={attention.sessionsDisconnected || Boolean(sessionListView.catalog?.failed)}
        view={sessionListView}
        onNewSession={options => sessionCreate.openForAgent("", options)}
      />
      <OsShortcutsDialog
        open={overlays.activeOverlay === "shortcuts"}
        onOpenChange={open => overlays.setOverlayOpen("shortcuts", open)}
      />
      <ProcessesLeftRunningHost
        open={overlays.activeOverlay === "left-running"}
        onOpenChange={open => overlays.setOverlayOpen("left-running", open)}
      />
      <OsAboutDialog
        open={overlays.activeOverlay === "about"}
        onOpenChange={open => overlays.setOverlayOpen("about", open)}
      />
      <OsWorkspacesOverview
        open={overlays.activeOverlay === "workspaces"}
        onOpenChange={open => overlays.setOverlayOpen("workspaces", open)}
        shortcutLabels={{
          picker: shortcutLabels.workspacePicker,
          globalScope: shortcutLabels.globalScope,
        }}
        workspaces={model.workspaces}
        activeWorkspaceId={model.activeWorkspaceId}
        onSelectWorkspace={workspaceId =>
          model.setActiveWorkspaceId(workspaceId, { scopeId: worktreeScopeId })
        }
        onNewWorkspace={model.openWorkspaceSetup}
        reducedMotion={reducedMotion}
        worktreesByWorkspace={worktreesByWorkspace}
        workingSessionsByWorkspace={workingSessionsByWorkspace}
        userHomeDir={model.userHomeDir}
        selectedWorktreeId={worktreeSelection.selectedWorktreeId}
        onSelectWorktree={(workspaceId, entry) => {
          if (worktreeDialogs.requestAdopt(workspaceId, entry)) return;
          if (entry.worktree) {
            selectWorktreeForScope(worktreeScopeId, workspaceId, entry.worktree.id);
          }
        }}
        onCreateWorktree={model.openWorktreeCreate}
        onResolveMissingWorktree={worktreeDialogs.requestResolveMissing}
        onRemoveWorktree={worktreeDialogs.requestRemove}
        onRemoveWorktrees={worktreeDialogs.requestRemoveBatch}
      />
      <WorkspaceSetupDialogBoundary
        defaults={workspaceSetupDefaults}
        onOpenChange={model.setWorkspaceSetupOpen}
        onWorkspaceResolved={model.setActiveWorkspaceId}
        open={model.isWorkspaceSetupOpen}
      />
      <DesktopWorktreeDialogs
        model={model}
        scopeId={worktreeScopeId}
        worktreeDialogs={worktreeDialogs}
      />
      <AgentCreateScreen
        instanceCapacity={model.agentCreate.instanceCapacity}
        draft={model.agentCreate.draft}
        duplicateSourceName={model.agentCreate.duplicateSourceName}
        hasActiveWorkspace={model.agentCreate.hasActiveWorkspace}
        isSubmitting={model.agentCreate.isSubmitting}
        modelCatalogError={model.agentCreate.modelCatalogError}
        modelCatalogLoading={model.agentCreate.modelCatalogLoading}
        modelCatalogRefreshing={model.agentCreate.modelCatalogRefreshing}
        onDraftChange={model.agentCreate.onDraftChange}
        onOpenChange={model.agentCreate.onOpenChange}
        onOpenCreatedAgent={model.agentCreate.onOpenCreatedAgent}
        onOpenProviderSettings={model.agentCreate.onOpenProviderSettings}
        onRefreshCatalog={model.agentCreate.onRefreshCatalog}
        onSubmit={model.agentCreate.onSubmit}
        open={model.agentCreate.open}
        providerOptions={model.agentCreate.providerOptions}
        providersError={model.agentCreate.providersError}
        providersLoading={model.agentCreate.providersLoading}
        runtimeModels={model.agentCreate.runtimeModels}
        startFailure={model.agentCreate.startFailure}
        submitError={model.agentCreate.submitError}
        takenNames={model.agentCreate.takenNames}
        workspaceName={model.agentCreate.workspaceName}
        workspaces={model.agentCreate.workspaces}
      />
      {/* Profile lifecycle dialogs live at the shell so a flow started from the
          command palette does not depend on Settings being open. */}
      <ProfileLifecycleHost onSetAutomationEnabled={setAutomationEnabled} />
    </div>
  );
}
