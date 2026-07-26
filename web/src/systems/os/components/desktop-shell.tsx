import { Outlet } from "@tanstack/react-router";

import { cn } from "@agh/ui";

import { AgentCreateDialog, AgentCreateHostProvider, useAgents } from "@/systems/agent";
import { useOnboardingStatus } from "@/systems/onboarding";
import { SessionCreateDialog, SessionCreateProvider } from "@/systems/session";
import { useSettingsSandboxes } from "@/systems/settings";
import {
  useWorkspaceSetupContent,
  WorkspaceOnboarding,
  WorkspaceSetupDialog,
  type WorkspaceSetupCollection,
  type WorkspaceSetupDefaultsModel,
} from "@/systems/workspace";

import { OsShellContext } from "../contexts/os-shell-context";
import { useDesktopChrome } from "../hooks/use-desktop-chrome";
import { useDesktopShellBody } from "../hooks/use-desktop-shell-body";
import { useDesktopShellModel, type DesktopShellModel } from "../hooks/use-desktop-shell-model";
import { DesktopGate } from "./desktop-gate";
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

/**
 * The desktop shell replaces the AppShell chrome (ADR-001): onboarding gate,
 * menubar, wallpapered win-layer, dock, ⌘K palette, and the window-manager
 * sync lifecycle. Route matches render through the (invisible) Outlet as
 * sync-controllers; windows render in the layer.
 */
export function DesktopShell() {
  // Same cached query the gate reads — no extra fetch, one truth for first run.
  const onboarding = useOnboardingStatus();
  const firstRun = onboarding.data?.completed === false;

  return (
    <DesktopGate>
      <DesktopChrome firstRun={firstRun} />
    </DesktopGate>
  );
}

function DesktopChrome({ firstRun }: { firstRun: boolean }) {
  const model = useDesktopShellModel();
  const chrome = useDesktopChrome(model.activeWorkspaceId);
  const agentsQuery = useAgents();
  const sandboxesQuery = useSettingsSandboxes();
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

  // First run legitimately has no workspace yet — setup is the one that adds it.
  // The post-setup workspace-less state still routes to workspace onboarding.
  if (!firstRun && !model.areWorkspacesLoading && !model.workspacesError && !model.hasWorkspaces) {
    return (
      <WorkspaceOnboardingBoundary
        defaults={workspaceSetupDefaults}
        onWorkspaceResolved={model.setActiveWorkspaceId}
      />
    );
  }

  return (
    <OsShellContext.Provider value={chrome.shell}>
      <SessionCreateProvider
        value={{
          openForAgent: model.sessionCreate.openForAgent,
          isCreating: model.sessionCreate.isSubmitting,
          pendingAgentName: model.sessionCreate.pendingAgentName,
          hasActiveWorkspace: model.activeWorkspace !== undefined,
        }}
      >
        <AgentCreateHostProvider
          openDialog={model.agentCreate.openDialog}
          openForDuplicate={model.agentCreate.openForDuplicate}
        >
          <DesktopShellBody
            firstRun={firstRun}
            model={model}
            workspaceSetupDefaults={workspaceSetupDefaults}
          />
        </AgentCreateHostProvider>
      </SessionCreateProvider>
    </OsShellContext.Provider>
  );
}

function DesktopShellBody({
  model,
  firstRun,
  workspaceSetupDefaults,
}: {
  model: DesktopShellModel;
  firstRun: boolean;
  workspaceSetupDefaults: WorkspaceSetupDefaultsModel;
}) {
  const {
    attention,
    desktop,
    desktopRef,
    gesturePreview,
    manager,
    managerSurfaces,
    onResize,
    onSeamPreview,
    onSeamPreviewEnd,
    overlays,
    pager,
    reducedMotion,
    transition,
    windowManagerActions,
    winLayer,
    workspaceDetails,
  } = useDesktopShellBody(model, { firstRun });

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
        onSelectWorkspace={model.setActiveWorkspaceId}
        onAddWorkspace={model.openWorkspaceSetup}
        onNewSession={() => model.sessionCreate.openForAgent("")}
        onOpenPalette={() => overlays.setOverlayOpen("palette", true)}
        onOpenDesktops={() => overlays.setOverlayOpen("desktops", true)}
        onOpenWorkspaces={() => overlays.setOverlayOpen("workspaces", true)}
        onToggleSessions={() => overlays.toggleOverlay("sessions")}
        activeOverlay={overlays.activeOverlay}
        onOverlayOpenChange={overlays.setOverlayOpen}
        attention={attention}
      />
      <div data-slot="os-desk" className="relative min-h-0 flex-1 overflow-hidden">
        <OsWallpaper wallpaper={desktop.wallpaper} />
        {Object.keys(desktop.windows).map(windowId => (
          <OsAppPreloader key={windowId} windowId={windowId} />
        ))}
        <OsWinLayer
          model={winLayer}
          reducedMotion={reducedMotion}
          transition={transition}
          preview={gesturePreview}
          onTransitionComplete={() => windowManagerActions.setTransitionIntent(null)}
          onResize={onResize}
          onSeamPreview={onSeamPreview}
          onSeamPreviewEnd={onSeamPreviewEnd}
        />
        <DesktopManagerSurfaces
          model={managerSurfaces}
          // The window manager binds to the active workspace, not to the catalog
          // being non-empty — a loaded catalog with no selection is still unbound.
          unbound={model.activeWorkspaceId === null}
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
          onOpenChange={open => {
            if (open) windowManagerActions.openOverlay({ kind: "desktops-overview" });
            else windowManagerActions.closeOverlay();
          }}
          onRetry={() => manager.refreshSnapshot()}
          onResolveConflict={() => {
            manager.clearConflict();
            manager.refreshSnapshot();
          }}
        />
        <DesktopDock
          dormant={firstRun}
          onNewSession={() => model.sessionCreate.openForAgent("")}
          badges={attention.badges}
          sessionsOpen={overlays.activeOverlay === "sessions"}
          onToggleSessions={() => overlays.toggleOverlay("sessions")}
          pager={
            <DesktopPagerSurface
              activeDesktopId={pager.activeDesktopId}
              desktops={pager.desktops}
              compact={pager.compact}
              canSwitchDesktop={pager.canSwitchDesktop}
              onSelectDesktop={desktopId => manager.switchDesktop(desktopId)}
              onOpenOverview={request => windowManagerActions.requestOverviewSegment(request)}
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
        sessions={attention.sessions}
        disconnected={attention.sessionsDisconnected}
      />
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
        workspaces={model.workspaces}
        activeWorkspaceId={model.activeWorkspaceId}
        onSelectWorkspace={model.setActiveWorkspaceId}
        onNewWorkspace={model.openWorkspaceSetup}
        details={workspaceDetails}
        presentation={pager.presentation}
        reducedMotion={reducedMotion}
      />
      <WorkspaceSetupDialogBoundary
        defaults={workspaceSetupDefaults}
        onOpenChange={model.setWorkspaceSetupOpen}
        onWorkspaceResolved={model.setActiveWorkspaceId}
        open={model.isWorkspaceSetupOpen}
      />
      <AgentCreateDialog
        draft={model.agentCreate.draft}
        hasActiveWorkspace={model.agentCreate.hasActiveWorkspace}
        isSubmitting={model.agentCreate.isSubmitting}
        modelCatalogError={model.agentCreate.modelCatalogError}
        modelCatalogLoading={model.agentCreate.modelCatalogLoading}
        modelCatalogLoaded={model.agentCreate.modelCatalogLoaded}
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
      <SessionCreateDialog
        agents={model.sessionCreate.agents}
        catalogError={model.sessionCreate.catalogError}
        catalogLoading={model.sessionCreate.catalogLoading}
        catalogLoaded={model.sessionCreate.catalogLoaded}
        catalogRefreshError={model.sessionCreate.catalogRefreshError}
        catalogRefreshing={model.sessionCreate.catalogRefreshing}
        catalogStale={model.sessionCreate.catalogStale}
        hasProviderOptions={model.sessionCreate.hasProviderOptions}
        isSubmitting={model.sessionCreate.isSubmitting}
        mode={model.sessionCreate.mode}
        networkParticipation={model.sessionCreate.networkParticipation}
        onAgentChange={model.sessionCreate.onAgentChange}
        onCatalogRefresh={model.sessionCreate.refreshCatalog}
        onModeChange={model.sessionCreate.onModeChange}
        onNetworkParticipationChange={model.sessionCreate.onNetworkParticipationChange}
        onOpenChange={model.sessionCreate.onOpenChange}
        onOpenProviderSettings={model.sessionCreate.openProviderSettings}
        onPromptChange={model.sessionCreate.onPromptChange}
        onRuntimeChange={model.sessionCreate.onRuntimeChange}
        onSessionNameChange={model.sessionCreate.onSessionNameChange}
        onSubmit={model.sessionCreate.submit}
        onWorkspaceChange={model.sessionCreate.onWorkspaceChange}
        onWorkspacePathChange={model.sessionCreate.onWorkspacePathChange}
        open={model.sessionCreate.open}
        promptValue={model.sessionCreate.promptValue}
        providersError={model.sessionCreate.providersError}
        providersLoading={model.sessionCreate.providersLoading}
        runtimeModels={model.sessionCreate.runtimeModels}
        runtimeProviders={model.sessionCreate.runtimeProviders}
        runtimeValue={model.sessionCreate.runtimeValue}
        selectedAgentName={model.sessionCreate.selectedAgentName}
        sessionName={model.sessionCreate.sessionName}
        submitError={model.sessionCreate.submitError}
        workspace={model.sessionCreate.workspace}
        workspaceId={model.sessionCreate.workspaceId}
        workspacePath={model.sessionCreate.workspacePath}
        workspaces={model.sessionCreate.workspaces}
      />
    </div>
  );
}

function WorkspaceOnboardingBoundary({
  defaults,
  onWorkspaceResolved,
}: {
  defaults: WorkspaceSetupDefaultsModel;
  onWorkspaceResolved: (workspaceId: string) => void;
}) {
  const setup = useWorkspaceSetupContent({ onWorkspaceResolved });
  return <WorkspaceOnboarding model={{ defaults, setup }} />;
}

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
