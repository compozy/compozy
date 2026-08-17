import { Outlet } from "@tanstack/react-router";

import { cn } from "@compozy/ui";

import { OsShellContext } from "../contexts/os-shell-context";
import { WorktreeDialogActionsContext } from "../contexts/worktree-dialog-actions-context";
import { useDesktopChrome } from "../hooks/use-desktop-chrome";
import { useDesktopShellBody } from "../hooks/use-desktop-shell-body";
import { useDesktopShellModel, type DesktopShellModel } from "../hooks/use-desktop-shell-model";
import { useDesktopWorktreeScope, WindowScopeContext } from "../hooks/use-worktree-scope";
import { useWorktreeDialogTargets } from "../hooks/use-worktree-dialog-targets";
import { DesktopGate } from "./desktop-gate";
import { DesktopWorktreeDialogs } from "./desktop-worktree-dialogs";
import { DesktopMenubar } from "./desktop-menubar";
import { DesktopDock } from "./desktop-dock";
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
import { AgentCreateDialog, AgentCreateHostProvider, useAgents } from "@/systems/agent";
import { useOnboardingStatus } from "@/systems/onboarding";
import {
  SessionCreateDialogHost,
  SessionCreateProvider,
  SessionDeleteDialog,
  SessionRenameDialog,
  useSessionCreateActions,
  useSessionLifecycleActions,
  useSessionListView,
} from "@/systems/session";
import {
  settingsUpdateIndicatorAvailable,
  useSettingsSandboxes,
  useSettingsUpdate,
} from "@/systems/settings";
import {
  selectWorktreeForScope,
  useWorkspaceSetupContent,
  WorkspaceSetupDialog,
  type WorkspaceSetupCollection,
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
  const model = useDesktopShellModel();
  const chrome = useDesktopChrome(model.runtimeWorkspaceId);
  const agentsQuery = useAgents();
  const sandboxesQuery = useSettingsSandboxes();
  const worktreeDialogs = useWorktreeDialogTargets();
  const workspaceSetupDefaults: WorkspaceSetupDefaultsModel = {
    agents: workspaceSetupCollection(
      agentsQuery.data,
      agentsQuery.isLoading,
      agentsQuery.error,
      "Could not load agents."
    ),
    sandboxes: workspaceSetupCollection(
      sandboxesQuery.data?.sandboxes.map(entry => ({
        name: entry.name,
        backend: entry.profile.backend,
      })),
      sandboxesQuery.isLoading,
      sandboxesQuery.error,
      "Could not load sandbox profiles."
    ),
  };

  // Zero project workspaces boots Global-on; the home registration is not a
  // project workspace and must not replace the shell with setup.

  return (
    <WorktreeDialogActionsContext.Provider value={worktreeDialogs}>
      <OsShellContext.Provider value={chrome.shell}>
        <SessionCreateProvider store={model.sessionCreate.store}>
          <AgentCreateHostProvider
            openDialog={model.agentCreate.openDialog}
            openForDuplicate={model.agentCreate.openForDuplicate}
          >
            <DesktopShellBody
              firstRun={firstRun}
              model={model}
              updateAvailable={updateAvailable}
              worktreeDialogs={worktreeDialogs}
              workspaceSetupDefaults={workspaceSetupDefaults}
            />
            <SessionCreateDialogHost
              activeWorkspace={model.runtimeWorkspace}
              agents={model.workspaceAgents}
              homeWorkspaceId={model.homeWorkspace?.id}
              projectWorkspaceId={model.activeWorkspaceId}
              scope={model.scope}
              store={model.sessionCreate.store}
            />
          </AgentCreateHostProvider>
        </SessionCreateProvider>
      </OsShellContext.Provider>
    </WorktreeDialogActionsContext.Provider>
  );
}

interface DesktopShellBodyProps {
  model: DesktopShellModel;
  firstRun: boolean;
  updateAvailable: boolean;
  workspaceSetupDefaults: WorkspaceSetupDefaultsModel;
  worktreeDialogs: ReturnType<typeof useWorktreeDialogTargets>;
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
  model,
  firstRun,
  updateAvailable,
  workspaceSetupDefaults,
  worktreeDialogs,
  worktreeScopeId,
  worktreeSelection,
}: DesktopShellBodyProps & DesktopWorktreeScope) {
  const sessionCreate = useSessionCreateActions();
  const sessionLifecycle = useSessionLifecycleActions({ workspaceId: model.runtimeWorkspaceId });
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
    reducedMotion,
    shortcutLabels,
    transition,
    winLayer,
    worktreesByWorkspace,
  } = useDesktopShellBody(model, {
    firstRun,
    onNewSession: openNewSession,
    sessionListView,
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
        deletionNotice={model.deletionNotice}
        rememberedWorkspaceName={model.rememberedWorkspace?.name ?? null}
        onToggleGlobalScope={model.toggleGlobalScope}
        onSelectWorkspace={workspaceId =>
          model.setActiveWorkspaceId(workspaceId, { scopeId: worktreeScopeId })
        }
        onAddWorkspace={model.openWorkspaceSetup}
        onNewSession={openNewSession}
        onOpenPalette={() => overlays.setOverlayOpen("palette", true)}
        onOpenDesktops={() => overlays.setOverlayOpen("desktops", true)}
        onOpenWorkspaces={() => overlays.setOverlayOpen("workspaces", true)}
        onToggleSessions={() => overlays.toggleOverlay("sessions")}
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
          // The window manager binds to the active workspace, not to the catalog
          // being non-empty — a loaded catalog with no selection is still unbound.
          // While resolution is pending nothing is claimable either way.
          unbound={!model.pending && model.runtimeWorkspaceId === null}
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
        <DesktopDock
          dormant={firstRun}
          onNewSession={openNewSession}
          badges={attention.badges}
          sessionsOpen={overlays.activeOverlay === "sessions"}
          contextMenusEnabled={overlays.activeOverlay === null}
          onToggleSessions={() => overlays.toggleOverlay("sessions")}
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
        open={overlays.activeOverlay === "palette"}
        onOpenChange={open => overlays.setOverlayOpen("palette", open)}
        onOpenDesktops={() => overlays.setOverlayOpen("desktops", true)}
        onToggleSessions={() => overlays.toggleOverlay("sessions")}
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

function workspaceSetupCollection<T>(
  entries: T[] | undefined,
  isLoading: boolean,
  error: unknown,
  fallbackMessage: string
): WorkspaceSetupCollection<T> {
  if (isLoading) return { state: "loading" };
  if (error) {
    return {
      state: "error",
      message:
        error instanceof Error && error.message.trim() !== "" ? error.message : fallbackMessage,
    };
  }
  return { state: "ready", entries: entries ?? [] };
}
