import { ChevronRight } from "@compozy/ui/icons";

import { Button, DescriptionCard, Panel } from "@compozy/ui";

import type { AgentInstructionsTabViewModel } from "../hooks/use-agent-instructions-tab";
import type { AgentInstructionFile } from "../lib/agent-detail-search";
import type { AgentPayload } from "../types";
import { AgentAuthoredFileEditor } from "./agent-authored-file-editor";
import { AgentHeartbeatOps } from "./agent-heartbeat-ops";
import { AgentHeartbeatOpsSkeleton } from "./agent-heartbeat-ops-skeleton";
import { InstructionsFileRow } from "./agent-instructions-file-row";

export type { AgentInstructionsTabViewModel };

export interface AgentInstructionsTabProps {
  agent: AgentPayload;
  file: AgentInstructionFile;
  onFileChange: (file: AgentInstructionFile) => void;
  onEditAgentPrompt: () => void;
  viewModel: AgentInstructionsTabViewModel;
  onNewSession: () => void;
}

export function AgentInstructionsTab({
  agent,
  file,
  onFileChange,
  onEditAgentPrompt,
  viewModel: page,
  onNewSession,
}: AgentInstructionsTabProps) {
  return (
    <div className="flex flex-col gap-3.5" data-testid="agent-instructions-tab">
      <InstructionsFileRow file={file} onFileChange={onFileChange}>
        {file === "agent" ? (
          <Button
            type="button"
            size="sm"
            variant="link"
            onClick={onEditAgentPrompt}
            data-testid="agent-file-edit-prompt"
          >
            Edit
            <ChevronRight aria-hidden="true" data-icon="inline-end" />
          </Button>
        ) : null}
      </InstructionsFileRow>

      {file === "agent" ? (
        <Panel bodyClassName="p-0" data-testid="agent-file-agent">
          <DescriptionCard bare className="px-5 py-4" data-testid="agent-file-prompt">
            {agent.prompt || "No prompt."}
          </DescriptionCard>
        </Panel>
      ) : null}

      {file === "soul" ? (
        <AgentAuthoredFileEditor
          key={page.soul.resourceKey}
          resourceKey={page.soul.resourceKey}
          kind="soul"
          payload={page.soul.payload}
          isLoading={page.soul.isLoading}
          isError={page.soul.isError}
          history={page.soul.history}
          onValidate={page.soul.onValidate}
          onSave={page.soul.onSave}
          onRestore={page.soul.onRestore}
          onRetry={page.soul.onRetry}
        />
      ) : null}

      {file === "heartbeat" ? (
        <AgentAuthoredFileEditor
          key={page.heartbeat.resourceKey}
          resourceKey={page.heartbeat.resourceKey}
          kind="heartbeat"
          payload={page.heartbeat.payload}
          isLoading={page.heartbeat.isLoading}
          isError={page.heartbeat.isError}
          history={page.heartbeat.history}
          headerSlot={
            page.heartbeat.statusLoading && !page.heartbeat.status ? (
              <AgentHeartbeatOpsSkeleton />
            ) : (
              <AgentHeartbeatOps
                status={page.heartbeat.status}
                statusLoading={page.heartbeat.statusLoading}
                statusError={page.heartbeat.statusError}
                onRetryStatus={page.heartbeat.onRetryStatus}
                activeSessions={page.heartbeat.activeSessions}
                selectedSessionId={page.heartbeat.wakeSessionId}
                onSelectSessionId={page.heartbeat.setWakeSessionId}
                onWake={page.heartbeat.onWake}
                waking={page.heartbeat.waking}
                wakeDecision={page.heartbeat.wakeDecision}
                wakeError={page.heartbeat.wakeError}
                onNewSession={onNewSession}
              />
            )
          }
          onValidate={page.heartbeat.onValidate}
          onSave={page.heartbeat.onSave}
          onRestore={page.heartbeat.onRestore}
          onRetry={page.heartbeat.onRetry}
        />
      ) : null}
    </div>
  );
}
