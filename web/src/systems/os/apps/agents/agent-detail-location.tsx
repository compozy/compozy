import { AlertCircle, Compass } from "lucide-react";
import { useDeferredValue, ViewTransition, type ReactNode } from "react";

import {
  Button,
  cn,
  Empty,
  LaneTabs,
  PAGE_CONTENT_GUTTER,
  SkeletonRegion,
  TabsContent,
  VIEW_TRANSITION_CLASS,
  type LaneTabsItem,
} from "@compozy/ui";

import {
  AGENT_DETAIL_BODY_CLASS,
  AGENT_DETAIL_PAGE_CLASS,
  AGENT_DETAIL_ROOT_CLASS,
  AGENT_DETAIL_TABS_CLASS,
  AGENT_TAB_PANEL_CLASS,
} from "./agent-detail-frame-classes";
import { agentDetailTabItems, agentSessionsStatus } from "./agent-detail-view";
import { listPaginationStatus } from "./list-pagination-status";
import { useAgentDetail, type UseAgentDetailResult } from "./use-agent-detail";
import { useAgentDetailTopbar } from "./use-agent-detail-topbar";
import {
  AgentConfigurationSkeleton,
  AgentConfigurationTab,
  AgentDetailHeader,
  AgentDetailHeaderSkeleton,
  type AgentDetailSearch,
  type AgentDetailTab,
  AgentDiagnosticsBanner,
  type AgentInstructionFile,
  AgentInstructionsSkeleton,
  AgentInstructionsTab,
  AgentOverviewSkeleton,
  AgentOverviewTab,
  type AgentPayload,
  AgentRuntimeControl,
  AgentSessionsTab,
  AgentSessionsTabSkeleton,
  useAgentInstructionsTab,
} from "@/systems/agent";
import { SessionDeleteDialog, SessionRenameDialog, type SessionPayload } from "@/systems/session";
import { OsSessionsDeriveHost } from "../../components/os-sessions-derive-host";
import { useAgentWindowScope } from "@/systems/agent";

interface AgentInstructionsSectionProps {
  agent: AgentPayload;
  file: AgentInstructionFile;
  onFileChange: (file: AgentInstructionFile) => void;
  onEditAgentPrompt: () => void;
  workspaceId: string | null;
  sessions: SessionPayload[];
  onNewSession: () => void;
}

function AgentInstructionsSection({
  agent,
  file,
  onFileChange,
  onEditAgentPrompt,
  workspaceId,
  sessions,
  onNewSession,
}: AgentInstructionsSectionProps) {
  const viewModel = useAgentInstructionsTab({ agent, file, workspaceId, sessions });
  return (
    <AgentInstructionsTab
      agent={agent}
      file={file}
      onFileChange={onFileChange}
      onEditAgentPrompt={onEditAgentPrompt}
      viewModel={viewModel}
      onNewSession={onNewSession}
    />
  );
}

interface AgentDetailFrameProps {
  /** Test id of the page root: the loaded page and its skeleton are told apart by it. */
  testId: string;
  diagnostics?: ReactNode;
  header: ReactNode;
  tabItems: LaneTabsItem<AgentDetailTab>[];
  tab: AgentDetailTab;
  onTabChange: (tab: AgentDetailTab) => void;
  /** The tab panels. */
  children: ReactNode;
}

/**
 * The agent page chrome: header meta, the tab list and the scrolling tab body. The loaded page and
 * its loading state both render it, so the tab bar and body hold the same place when the agent
 * arrives.
 */
function AgentDetailFrame({
  testId,
  diagnostics,
  header,
  tabItems,
  tab,
  onTabChange,
  children,
}: AgentDetailFrameProps) {
  // The tab lives in the window route, an external store whose updates are never transitions, so a
  // `ViewTransition` could not see the swap. A deferred read turns it into one: the old panel
  // cross-fades into the new one while the tab list stays put.
  const activeTab = useDeferredValue(tab);
  return (
    <div className={AGENT_DETAIL_ROOT_CLASS} data-testid={testId}>
      <div className={AGENT_DETAIL_PAGE_CLASS}>
        {diagnostics}
        {header}
        <LaneTabs
          ariaLabel="Agent detail panels"
          className={AGENT_DETAIL_TABS_CLASS}
          items={tabItems}
          listClassName="w-full"
          onChange={onTabChange}
          value={activeTab}
          data-testid="agent-detail-tabs"
        >
          <ViewTransition default="none" update={VIEW_TRANSITION_CLASS.fade}>
            <div className={AGENT_DETAIL_BODY_CLASS} data-testid="agent-detail-body">
              {children}
            </div>
          </ViewTransition>
        </LaneTabs>
      </div>
    </div>
  );
}

/** The page while the agent loads: the real frame and tab list over the skeleton of the open tab. */
function AgentDetailLoading({ page }: { page: UseAgentDetailResult }) {
  const { tab, file, filter } = page.search;
  return (
    <AgentDetailFrame
      header={<AgentDetailHeaderSkeleton />}
      onTabChange={page.setTab}
      tab={tab}
      tabItems={agentDetailTabItems(page)}
      testId="agent-detail-loading"
    >
      <TabsContent value="overview" className={AGENT_TAB_PANEL_CLASS}>
        <SkeletonRegion label="Loading agent">
          <AgentOverviewSkeleton />
        </SkeletonRegion>
      </TabsContent>
      <TabsContent value="instructions" className={AGENT_TAB_PANEL_CLASS}>
        <AgentInstructionsSkeleton file={file} onFileChange={page.setFile} />
      </TabsContent>
      <TabsContent value="configuration" className={AGENT_TAB_PANEL_CLASS}>
        <SkeletonRegion label="Loading configuration">
          <AgentConfigurationSkeleton />
        </SkeletonRegion>
      </TabsContent>
      <TabsContent value="sessions" className={AGENT_TAB_PANEL_CLASS}>
        <AgentSessionsTabSkeleton filter={filter} onFilterChange={page.setFilter} />
      </TabsContent>
    </AgentDetailFrame>
  );
}

function AgentDetailNotFound({
  name,
  error,
  onBack,
}: {
  name: string;
  error: Error | null;
  onBack: () => void;
}) {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}>
      <Empty
        icon={AlertCircle}
        title="Agent not found"
        description={error?.message ?? `No agent named "${name}" was found in this project.`}
        action={
          <Button
            type="button"
            variant="secondary"
            size="sm"
            onClick={onBack}
            data-testid="agent-detail-back-agents"
          >
            <Compass aria-hidden="true" />
            Back to agents
          </Button>
        }
        data-testid="agent-detail-not-found"
      />
    </div>
  );
}

function AgentDiagnosticsSlot({ agent }: { agent: AgentPayload }) {
  if (!agent.diagnostics || agent.diagnostics.length === 0) return null;
  return (
    <div className="shrink-0 pt-5" data-testid="agent-diagnostics-slot">
      <AgentDiagnosticsBanner diagnostics={agent.diagnostics} />
    </div>
  );
}

function AgentSessionDialogs({ page }: { page: UseAgentDetailResult }) {
  const { sessionDeleteDialog: deleteDialog, sessionRenameDialog: renameDialog } = page;
  return (
    <>
      {deleteDialog.session ? (
        <SessionDeleteDialog
          open={deleteDialog.open}
          onOpenChange={deleteDialog.onOpenChange}
          session={deleteDialog.session}
          sessions={deleteDialog.sessions}
          results={deleteDialog.results}
          onRetry={deleteDialog.onRetry}
          isDeleting={deleteDialog.isDeleting}
          onConfirm={deleteDialog.onConfirm}
        />
      ) : null}
      {renameDialog.session ? (
        <SessionRenameDialog
          open={renameDialog.open}
          onOpenChange={renameDialog.onOpenChange}
          session={renameDialog.session}
          isRenaming={renameDialog.isRenaming}
          onConfirm={renameDialog.onConfirm}
        />
      ) : null}
    </>
  );
}

interface AgentSessionsPanelProps {
  page: UseAgentDetailResult;
  name: string;
  workspaceId: string | null;
}

function AgentSessionsPanel({ page, name, workspaceId }: AgentSessionsPanelProps) {
  return (
    <>
      <OsSessionsDeriveHost workspaceId={workspaceId}>
        {() => (
          <AgentSessionsTab
            agentName={name}
            sessions={page.sessions}
            archivedSessions={page.archivedSessions}
            archivedTotal={page.archivedSessionsTotal}
            total={page.sessionsTotal}
            active={page.activeSessionsTotal}
            failed={page.failedSessionsTotal}
            metricsUnavailable={page.metricsUnavailable}
            metricsLoading={page.metricsLoading}
            status={agentSessionsStatus(page)}
            paginationStatus={listPaginationStatus(
              page.isLoadingMoreSessions,
              page.hasMoreSessions
            )}
            onLoadMore={page.onLoadMoreSessions}
            archivedPaginationStatus={listPaginationStatus(
              page.isLoadingMoreArchivedSessions,
              page.hasMoreArchivedSessions
            )}
            onLoadMoreArchived={page.onLoadMoreArchivedSessions}
            sessionActions={page.sessionActions}
            filter={page.search.filter}
            onFilterChange={page.setFilter}
            onNewSession={page.onNewSession}
            onClearFilter={() => page.setFilter("all")}
            onRetry={page.onRetrySessions}
          />
        )}
      </OsSessionsDeriveHost>
      <AgentSessionDialogs page={page} />
    </>
  );
}

interface AgentDetailBodyProps {
  agent: AgentPayload;
  name: string;
  page: UseAgentDetailResult;
  workspaceId: string | null;
}

function AgentDetailBody({ agent, name, page, workspaceId }: AgentDetailBodyProps) {
  return (
    <>
      {page.deleteDialog}
      <AgentDetailLoadedFrame agent={agent} name={name} page={page} workspaceId={workspaceId} />
    </>
  );
}

function AgentDetailLoadedFrame({ agent, name, page, workspaceId }: AgentDetailBodyProps) {
  return (
    <AgentDetailFrame
      diagnostics={<AgentDiagnosticsSlot agent={agent} />}
      header={<AgentDetailHeader agent={agent} />}
      onTabChange={page.setTab}
      tab={page.search.tab}
      tabItems={agentDetailTabItems(page)}
      testId="agent-detail-page"
    >
      <TabsContent value="overview" className={AGENT_TAB_PANEL_CLASS}>
        <AgentOverviewTab
          agent={agent}
          sessions={page.sessions}
          sessionsTotal={page.sessionsTotal}
          activeSessionsTotal={page.activeSessionsTotal}
          failedSessionsTotal={page.failedSessionsTotal}
          runtimeSeconds={page.runtimeSeconds}
          metricsUnavailable={page.metricsUnavailable}
          metricsLoading={page.metricsLoading}
          lastSessionActivityAt={page.lastSessionActivityAt}
          sessionsLoading={page.sessionsLoading}
          sessionsError={page.sessionsError}
          runtimeControl={
            <AgentRuntimeControl
              agent={agent}
              labelledBy="agent-overview-model-label"
              workspaceId={workspaceId}
            />
          }
          onEditRuntime={() => page.onEditSettings("runtime")}
          onViewAllSessions={() => page.setTab("sessions")}
        />
      </TabsContent>

      <TabsContent value="instructions" className={AGENT_TAB_PANEL_CLASS}>
        <AgentInstructionsSection
          agent={agent}
          file={page.search.file}
          onFileChange={page.setFile}
          onEditAgentPrompt={() => page.onEditSettings("instructions")}
          workspaceId={workspaceId}
          sessions={page.sessions}
          onNewSession={page.onNewSession}
        />
      </TabsContent>

      <TabsContent value="configuration" className={AGENT_TAB_PANEL_CLASS}>
        <AgentConfigurationTab
          agent={agent}
          onEditSection={section => page.onEditSettings(section)}
        />
      </TabsContent>

      <TabsContent value="sessions" className={AGENT_TAB_PANEL_CLASS}>
        <AgentSessionsPanel page={page} name={name} workspaceId={workspaceId} />
      </TabsContent>
    </AgentDetailFrame>
  );
}

interface AgentDetailContentProps {
  name: string;
  rawSearch: AgentDetailSearch;
}

/** Present an agent and its sessions, forwarding lifecycle results to the shared confirmation UI. */
export function AgentDetailLocation({ name, rawSearch }: AgentDetailContentProps) {
  const page = useAgentDetail(name, rawSearch);
  const { projectId } = useAgentWindowScope();
  useAgentDetailTopbar(page, name);

  if (page.agentLoading) {
    return <AgentDetailLoading page={page} />;
  }

  if (page.agentError || !page.agent) {
    return <AgentDetailNotFound name={name} error={page.agentError} onBack={page.onBackToAgents} />;
  }

  return <AgentDetailBody agent={page.agent} name={name} page={page} workspaceId={projectId} />;
}
