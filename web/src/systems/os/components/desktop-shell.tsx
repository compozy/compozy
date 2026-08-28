import { Outlet } from "@tanstack/react-router";

import { cn } from "@compozy/ui";

import { CmdPaletteRegistryProvider } from "../contexts/cmd-palette-registry-context";
import { OsShellContext } from "../contexts/os-shell-context";
import { useCmdPaletteRegistry } from "../hooks/use-cmd-palette-registry";
import { WorktreeDialogActionsContext } from "../contexts/worktree-dialog-actions-context";
import { useDesktopChromeController } from "../hooks/use-desktop-chrome-controller";
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
import { ShellDesktopDock } from "./desktop-dock";
import { DesktopManagerSurfaces } from "./desktop-manager-surfaces";
import { DesktopPagerSurface } from "./desktop-pager-surface";
import { OsAboutDialog } from "./os-about-dialog";
import { OsAppPreloader } from "./os-app-preloader";
import { OsCommandPalette } from "./os-command-palette";
import { OsShortcutsDialog } from "./os-shortcuts-dialog";
import { OsWorkspacesOverview } from "./os-workspaces-overview";
import { OsWallpaper } from "./os-wallpaper";
import { OsWinLayer } from "./os-win-layer";
import { OsSessionsModal } from "./sessions-modal";
import { AgentCreateDialog, AgentCreateHostProvider } from "@/systems/agent";
import { useOnboardingStatus } from "@/systems/onboarding";
import {
  ProfileLifecycleHost,
  ProfileSwitcherSlot,
  WorkspaceProfilesHint,
} from "@/systems/profiles";
import {
  SessionCreateDialogHost,
  SessionCreateProvider,
  SessionDeleteDialog,
  SessionRenameDialog,
  useSessionCreateActions,
  useSessionLifecycleActions,
  useSessionListView,
} from "@/systems/session";
import { settingsUpdateIndicatorAvailable, useSettingsUpdate } from "@/systems/settings";
import {
  selectWorktreeForScope,
  useWorkspaceSetupContent,
  WorkspaceSetupDialog,
  type WorkspaceSetupDefaultsModel,
} from "@/systems/workspace";

/**
 * The desktop shell replaces the AppShell chrome (ADR-001): onboarding gate,
 * menubar, wallpapered win-layer, dock, ⌘K palette, and the window-manager
 * sync lifecycle. Route matches render through the (invisible) Outlet as
 * sync-controllers; windows render in the layer.
 */
export function DesktopShell() {
  // Same cached query the gate reads — no extra fetch, one truth for first run.
  const onboarding = useOnboardingStatus();
  // Same cached read the Updates section uses — one truth, one query key.
  const update = useSettingsUpdate();
  const firstRun = onboarding.data?.completed === false;

  return (
    <DesktopGate>
      <DesktopChrome
        firstRun={firstRun}
        updateAvailable={settingsUpdateIndicatorAvailable(update.data)}
      />
    </DesktopGate>
  );
}

function DesktopChrome({
  firstRun,
  updateAvailable,
}: {
  firstRun: boolean;
  updateAvailable: boolean;
}) {
  const controller = useDesktopChromeController();

  // Zero project workspaces boots Global-on; the home registration is not a
  // project workspace and must not replace the shell with setup.

  return (
    <WorktreeDialogActionsContext.Provider value={controller.worktreeDialogs}>
      <OsShellContext.Provider value={controller.chrome.shell}>
        <DesktopChromeContent
          client={controller.chrome.client}
          firstRun={firstRun}
          continuityStreamsEnabled={controller.continuityStreamsEnabled}
          model={controller.model}
          clientCommandChannel={controller.chrome.clientCommandChannel}
          updateAvailable={updateAvailable}
          worktreeDialogs={controller.worktreeDialogs}
          workspaceSetupDefaults={controller.workspaceSetupDefaults}
        />
      </OsShellContext.Provider>
    </WorktreeDialogActionsContext.Provider>
  );
}

interface DesktopShellBodyProps {
  continuityStreamsEnabled: boolean;
  client: WindowManagerRegisteredClientView | null;
  model: DesktopShellModel;
  firstRun: boolean;
  updateAvailable: boolean;
  workspaceSetupDefaults: WorkspaceSetupDefaultsModel;
  worktreeDialogs: WorktreeDialogTargets;
  /** The daemon's client-command channel reads the current shell seam through this port. */
  clientCommandChannel: ClientCommandChannel;
}

function DesktopChromeContent(props: DesktopShellBodyProps) {
  // This projection reads the desktop topology, so it must mount below the
  // shell context that owns the window-manager store.
  const paletteRegistry = useCmdPaletteRegistry({
    // The structural catalog belongs to the attached desktop client. Global
    // changes the command lens, but it must not detach shell commands from the
    // remembered project's window-manager partition.
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
            homeWorkspaceId={props.model.homeWorkspace?.id}
            projectWorkspaceId={props.model.activeWorkspaceId}
            scope={props.model.scope}
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

function DesktopShellScopedBody({
  continuityStreamsEnabled,
  client,
  model,
  firstRun,
  updateAvailable,
  workspaceSetupDefaults,
  worktreeDialogs,
  worktreeScopeId,
  worktreeSelection,
  clientCommandChannel,
}: DesktopShellBodyProps & DesktopWorktreeScope) {
  const sessionCreate = useSessionCreateActions();
  const sessionLifecycle = useSessionLifecycleActions({ workspaceId: model.runtimeWorkspaceId });
  const setAutomationEnabled = useProfileAutomationEnablement();
  // Scope and order are the operator's, persisted by the daemon; the modal
  // renders them rather than fetching its own.
  const sessionListView = useSessionListView();
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
    winLayer,
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
      className="flex min-h-0 flex-1 flex-col overflow-hidden focus-visible:shadow-focus-inset focus-visible:outline-none"
    >
      <DesktopMenubar
        profileSwitcher={
          <ProfileSwitcherSlot
            onOpenSettings={() => void paletteDispatch.runById("settings.profiles")}
          />
        }
        // Dimmed while setup blocks: readable enough to see what you unlock,
        // never bright enough to read as available.
        className={cn(
          "transition-opacity duration-shell-slow motion-reduce:transition-none",
          firstRun && "opacity-68"
        )}
        workspaces={model.workspaces}
        activeWorkspace={model.activeWorkspace}
        chip={model.chip}
        scope={model.scope}
        scopePending={model.pending}
        toggleLocked={model.toggleLocked}
        canDisableGlobal={model.canDisableGlobal}
        rememberedWorkspaceName={model.rememberedWorkspace?.name ?? null}
        onToggleGlobalScope={model.toggleGlobalScope}
        onSelectWorkspace={workspaceId =>
          model.setActiveWorkspaceId(workspaceId, { scopeId: worktreeScopeId })
        }
        onAddWorkspace={model.openWorkspaceSetup}
        onRunCommand={commandId => void paletteDispatch.runById(commandId)}
        activeOverlay={overlays.activeOverlay}
        onOverlayOpenChange={overlays.setOverlayOpen}
        attention={attention}
        updateAvailable={updateAvailable}
        worktreesByWorkspace={worktreesByWorkspace}
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
      />
      <div data-slot="os-desk" className="relative min-h-0 flex-1 overflow-hidden">
        <OsWallpaper wallpaper={desktop.wallpaper} />
        {model.activeWorkspaceId !== null ? (
          <WorkspaceProfilesHint
            hints={model.workspaceProfileHints}
            workspaceId={model.activeWorkspaceId}
          />
        ) : null}
        {Object.keys(desktop.windows).map(windowId => (
          <OsAppPreloader key={windowId} windowId={windowId} />
        ))}
        <OsWinLayer
          model={winLayer}
          paletteShortcutLabel={shortcutLabels.palette}
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
          // Global changes the data lens, not the desktop layout partition. The
          // window manager stays on the remembered project while data aggregates.
          unbound={!model.pending && model.desktopWorkspaceId === null}
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
        <ShellDesktopDock
          dormant={firstRun}
          onNewSession={openNewSession}
          badges={attention.badges}
          contextMenusEnabled={overlays.activeOverlay === null}
          pager={
            <DesktopPagerSurface
              activeDesktopId={pager.activeDesktopId}
              desktops={pager.desktops}
              compact={pager.compact}
              canSwitchDesktop={pager.canSwitchDesktop}
              onSelectDesktop={desktopId => manager.switchDesktop(desktopId)}
              onOpenOverview={onOpenDesktopOverview}
            />
          }
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
      <OsSessionsModal
        open={overlays.activeOverlay === "sessions"}
        onOpenChange={open => overlays.setOverlayOpen("sessions", open)}
        dismissalBlocked={sessionLifecycle.deleteDialog.open || sessionLifecycle.renameDialog.open}
        sessions={attention.sessions}
        disconnected={attention.sessionsDisconnected}
        view={sessionListView}
        currentWorkspaceId={model.runtimeWorkspaceId}
        onNewSession={openNewSession}
        sessionActions={sessionLifecycle.actions}
      />
      {sessionLifecycle.deleteDialog.session ? (
        <SessionDeleteDialog
          open={sessionLifecycle.deleteDialog.open}
          onOpenChange={sessionLifecycle.deleteDialog.onOpenChange}
          session={sessionLifecycle.deleteDialog.session}
          isDeleting={sessionLifecycle.deleteDialog.isDeleting}
          onConfirm={sessionLifecycle.deleteDialog.onConfirm}
        />
      ) : null}
      {sessionLifecycle.renameDialog.session ? (
        <SessionRenameDialog
          open={sessionLifecycle.renameDialog.open}
          onOpenChange={sessionLifecycle.renameDialog.onOpenChange}
          session={sessionLifecycle.renameDialog.session}
          isRenaming={sessionLifecycle.renameDialog.isRenaming}
          onConfirm={sessionLifecycle.renameDialog.onConfirm}
        />
      ) : null}
      <OsShortcutsDialog
        open={overlays.activeOverlay === "shortcuts"}
        onOpenChange={open => overlays.setOverlayOpen("shortcuts", open)}
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
        scope={model.scope}
        onSelectWorkspace={workspaceId =>
          model.setActiveWorkspaceId(workspaceId, { scopeId: worktreeScopeId })
        }
        onNewWorkspace={model.openWorkspaceSetup}
        reducedMotion={reducedMotion}
        worktreesByWorkspace={worktreesByWorkspace}
        userHomeDir={model.userHomeDir}
        selectedWorktreeId={worktreeSelection.selectedWorktreeId}
        onSelectWorktree={(workspaceId, entry) => {
          if (worktreeDialogs.requestAdopt(workspaceId, entry)) return;
          if (entry.worktree) {
            selectWorktreeForScope(worktreeScopeId, workspaceId, entry.worktree.id);
          }
        }}
        onCreateWorktree={model.openWorktreeCreate}
        onRemoveWorktree={worktreeDialogs.requestRemove}
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
      <AgentCreateDialog
        draft={model.agentCreate.draft}
        hasActiveWorkspace={model.agentCreate.hasActiveWorkspace}
        isSubmitting={model.agentCreate.isSubmitting}
        modelCatalogError={model.agentCreate.modelCatalogError}
        modelCatalogLoading={model.agentCreate.modelCatalogLoading}
        modelCatalogRefreshing={model.agentCreate.modelCatalogRefreshing}
        onDraftChange={model.agentCreate.onDraftChange}
        onOpenChange={model.agentCreate.onOpenChange}
        onOpenProviderSettings={model.agentCreate.onOpenProviderSettings}
        onRefreshCatalog={model.agentCreate.onRefreshCatalog}
        onSubmit={model.agentCreate.onSubmit}
        open={model.agentCreate.open}
        providerOptions={model.agentCreate.providerOptions}
        providersError={model.agentCreate.providersError}
        providersLoading={model.agentCreate.providersLoading}
        runtimeModels={model.agentCreate.runtimeModels}
        submitError={model.agentCreate.submitError}
        workspaceId={model.agentCreate.workspaceId}
        workspaceName={model.agentCreate.workspaceName}
      />
      {/* Profile lifecycle dialogs live at the shell so a flow started from the
          command palette does not depend on Settings being open. */}
      <ProfileLifecycleHost
        enabled={continuityStreamsEnabled}
        onSetAutomationEnabled={setAutomationEnabled}
      />
    </div>
  );
}

/**
 * Mirrors `WorkspaceSetupDialogBoundary`: the domain hook runs in exactly one
 * place and only while the dialog is mounted, so a closed dialog holds no
 * mutation state.
 */
function WorkspaceSetupDialogBoundary({
  defaults,
  onOpenChange,
  onWorkspaceResolved,
  open,
}: {
  defaults: WorkspaceSetupDefaultsModel;
  onOpenChange: (open: boolean) => void;
  onWorkspaceResolved: (workspaceId: string) => void;
  open: boolean;
}) {
  const setup = useWorkspaceSetupContent({
    onWorkspaceResolved,
    onSuccessClose: () => onOpenChange(false),
  });
  return (
    <WorkspaceSetupDialog model={{ defaults, setup }} onOpenChange={onOpenChange} open={open} />
  );
}
