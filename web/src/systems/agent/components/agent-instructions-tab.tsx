import {
  Button,
  DescriptionCard,
  Eyebrow,
  Pill,
  PillGroup,
  Skeleton,
  type PillGroupItem,
} from "@agh/ui";

import type { AgentInstructionsTabViewModel } from "../hooks/use-agent-instructions-tab";
import type { AgentInstructionFile } from "../lib/agent-detail-search";
import type { AgentPayload } from "../types";
import { AgentAuthoredFileEditor } from "./agent-authored-file-editor";
import { AgentHeartbeatOps } from "./agent-heartbeat-ops";
import { AgentPanelBox } from "./agent-panel-box";

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
  const fileItems: PillGroupItem<AgentInstructionFile>[] = [
    { value: "agent", label: <Eyebrow>AGENT.md</Eyebrow>, testId: "agent-file-tab-agent" },
    {
      value: "soul",
      label: (
        <span className="inline-flex items-center gap-1.5">
          <Eyebrow>SOUL.md</Eyebrow>
          {page.soulMissing ? (
            <Pill size="sm" tone="warning" data-testid="agent-file-soul-missing-badge">
              missing
            </Pill>
          ) : null}
        </span>
      ),
      testId: "agent-file-tab-soul",
    },
    {
      value: "heartbeat",
      label: (
        <span className="inline-flex items-center gap-1.5">
          <Eyebrow>HEARTBEAT.md</Eyebrow>
          {page.heartbeatMissing ? (
            <Pill size="sm" tone="warning" data-testid="agent-file-heartbeat-missing-badge">
              missing
            </Pill>
          ) : null}
        </span>
      ),
      testId: "agent-file-tab-heartbeat",
    },
  ];

  return (
    <div className="flex flex-col gap-3.5" data-testid="agent-instructions-tab">
      <div className="flex flex-wrap items-center gap-2.5">
        <PillGroup
          aria-label="Authored files"
          data-testid="agent-instructions-files"
          items={fileItems}
          onChange={onFileChange}
          size="md"
          value={file}
        />
        <span className="min-w-0 flex-1" />
        {file === "agent" ? (
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={onEditAgentPrompt}
            data-testid="agent-file-edit-prompt"
          >
            Edit ›
          </Button>
        ) : null}
      </div>

      {file === "agent" ? (
        <AgentPanelBox data-testid="agent-file-agent">
          <div
            className="flex items-center gap-2.5 border-b border-line-soft px-4 py-2.5 text-small-body text-muted"
            data-testid="agent-file-meta"
          >
            <span className="font-mono text-badge tracking-mono">{page.promptWordCount}</span>
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
            <span>Read-only here</span>
          </div>
          <DescriptionCard bare className="px-5 py-4" data-testid="agent-file-prompt">
            {agent.prompt || "No prompt."}
          </DescriptionCard>
        </AgentPanelBox>
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
              <Skeleton className="h-32 rounded-md" />
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
