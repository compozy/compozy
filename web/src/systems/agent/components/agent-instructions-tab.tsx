import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  DescriptionCard,
  Eyebrow,
  Panel,
  PillGroup,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  type PillGroupItem,
} from "@compozy/ui";

import type { AgentInstructionsTabViewModel } from "../hooks/use-agent-instructions-tab";
import type { AgentInstructionFile } from "../lib/agent-detail-search";
import type { AgentPayload } from "../types";
import { AgentAuthoredFileEditor } from "./agent-authored-file-editor";
import { AgentHeartbeatOps, AgentHeartbeatOpsSkeleton } from "./agent-heartbeat-ops";
import { AuthoredFileLoading } from "./agent-authored-file-parts";

export type { AgentInstructionsTabViewModel };

const FILE_ITEMS: PillGroupItem<AgentInstructionFile>[] = [
  { value: "agent", label: <Eyebrow>AGENT.md</Eyebrow>, testId: "agent-file-tab-agent" },
  {
    value: "soul",
    label: <Eyebrow>SOUL.md</Eyebrow>,
    testId: "agent-file-tab-soul",
  },
  {
    value: "heartbeat",
    label: <Eyebrow>HEARTBEAT.md</Eyebrow>,
    testId: "agent-file-tab-heartbeat",
  },
];

interface InstructionsFileRowProps {
  file: AgentInstructionFile;
  onFileChange: (file: AgentInstructionFile) => void;
  children?: ReactNode;
}

/** The authored-file pills with the file's trailing action, shared by the tab and its skeleton. */
function InstructionsFileRow({ file, onFileChange, children }: InstructionsFileRowProps) {
  return (
    <div className="flex flex-wrap items-center gap-2.5">
      <PillGroup
        aria-label="Authored files"
        data-testid="agent-instructions-files"
        items={FILE_ITEMS}
        onChange={onFileChange}
        size="md"
        value={file}
      />
      <span className="min-w-0 flex-1" />
      {children}
    </div>
  );
}

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

export type AgentInstructionsSkeletonProps = Pick<
  AgentInstructionsTabProps,
  "file" | "onFileChange"
> & {
  /** Takes the file pills out of reach, for a frame shown before the window mounts. */
  inert?: boolean;
};

/** The tab while the agent loads: the real file pills over the panel skeleton for that file. */
export function AgentInstructionsSkeleton({
  file,
  onFileChange,
  inert = false,
}: AgentInstructionsSkeletonProps) {
  const files = (
    <InstructionsFileRow file={file} onFileChange={onFileChange}>
      {file === "agent" ? <Skeleton className="h-button-sm w-12" /> : null}
    </InstructionsFileRow>
  );
  return (
    <div className="flex flex-col gap-3.5" data-testid="agent-instructions-skeleton">
      {inert ? (
        <div className="contents" inert>
          {files}
        </div>
      ) : (
        files
      )}
      {file === "agent" ? (
        <SkeletonRegion label="Loading AGENT.md">
          <Panel bodyClassName="p-0">
            <SkeletonText className="px-5 py-4 text-body" lines={6} />
          </Panel>
        </SkeletonRegion>
      ) : (
        <AuthoredFileLoading
          headerSlot={file === "heartbeat" ? <AgentHeartbeatOpsSkeleton /> : undefined}
          kind={file}
        />
      )}
    </div>
  );
}
