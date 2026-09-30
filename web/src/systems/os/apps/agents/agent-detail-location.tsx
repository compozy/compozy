import { AlertCircle, Compass } from "lucide-react";

import {
  Button,
  cn,
  Empty,
  LaneTabs,
  PAGE_CONTENT_GUTTER,
  Skeleton,
  TabsContent,
} from "@compozy/ui";

import { agentDetailTabItems, agentSessionsStatus } from "./agent-detail-view";
import { listPaginationStatus } from "./list-pagination-status";
import { useAgentDetail, type UseAgentDetailResult } from "./use-agent-detail";
import { useAgentDetailTopbar } from "./use-agent-detail-topbar";
import {
  AgentConfigurationTab,
  AgentDetailHeader,
  type AgentDetailSearch,
  AgentDiagnosticsBanner,
  type AgentInstructionFile,
  AgentInstructionsTab,
  AgentOverviewTab,
  type AgentPayload,
  AgentRuntimeControl,
  AgentSessionsTab,
  useAgentInstructionsTab,
} from "@/systems/agent";
import { SessionDeleteDialog, SessionRenameDialog, type SessionPayload } from "@/systems/session";
import { OsSessionsDeriveHost } from "../../components/os-sessions-derive-host";
import { useActiveWorkspace } from "@/systems/workspace";

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

function AgentDetailLoading() {
  return (
    <div
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col gap-4 py-5")}
      data-testid="agent-detail-loading"
    >
      <Skeleton className="h-10 w-64" />
      <Skeleton className="h-8 w-80" />
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => (
          <Skeleton key={index} className="h-20 rounded-md" />
        ))}
      </div>
      <Skeleton className="h-48 rounded-md" />
    </div>
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
    <div
      className="flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden"
      data-testid="agent-detail-page"
    >
      {page.deleteDialog}
      <div className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col")}>
        <AgentDiagnosticsSlot agent={agent} />
        <AgentDetailHeader agent={agent} />
        <LaneTabs
          ariaLabel="Agent detail panels"
          className="min-h-0 flex-1 gap-0"
          items={agentDetailTabItems(page)}
          listClassName="w-full"
          onChange={page.setTab}
          value={page.search.tab}
          data-testid="agent-detail-tabs"
        >
          <div
            // The inline 4px pad keeps card hairlines (shadow rings) clear of the scroll clip.
            className="-mx-1 flex min-h-0 flex-1 flex-col overflow-y-auto px-1 py-5"
            data-testid="agent-detail-body"
          >
            <TabsContent value="overview" className="flex flex-col gap-6">
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

            <TabsContent value="instructions" className="flex flex-col gap-6">
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

            <TabsContent value="configuration" className="flex flex-col gap-6">
              <AgentConfigurationTab
                agent={agent}
                onEditSection={section => page.onEditSettings(section)}
              />
            </TabsContent>

            <TabsContent value="sessions" className="flex flex-col gap-6">
              <AgentSessionsPanel page={page} name={name} workspaceId={workspaceId} />
            </TabsContent>
          </div>
        </LaneTabs>
      </div>
    </div>
  );
}

interface AgentDetailContentProps {
  name: string;
  rawSearch: AgentDetailSearch;
}

/** Present an agent and its sessions, forwarding lifecycle results to the shared confirmation UI. */
export function AgentDetailLocation({ name, rawSearch }: AgentDetailContentProps) {
  const page = useAgentDetail(name, rawSearch);
  const { runtimeWorkspaceId } = useActiveWorkspace();
  useAgentDetailTopbar(page, name);

  if (page.agentLoading) {
    return <AgentDetailLoading />;
  }

  if (page.agentError || !page.agent) {
    return <AgentDetailNotFound name={name} error={page.agentError} onBack={page.onBackToAgents} />;
  }

  return (
    <AgentDetailBody agent={page.agent} name={name} page={page} workspaceId={runtimeWorkspaceId} />
  );
}
