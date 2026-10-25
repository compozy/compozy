import { Panel, Skeleton, SkeletonRegion, SkeletonText } from "@compozy/ui";

import type { AgentInstructionFile } from "../lib/agent-detail-search";
import {
  AgentHeartbeatOpsPlaceholder,
  AgentHeartbeatOpsSkeleton,
} from "./agent-heartbeat-ops-skeleton";
import { AuthoredFileLoading, AuthoredFilePanelPlaceholder } from "./agent-authored-file-loading";
import { InstructionsFileRow } from "./agent-instructions-file-row";

export interface AgentInstructionsSkeletonProps {
  file: AgentInstructionFile;
  onFileChange: (file: AgentInstructionFile) => void;
}

/** The tab while the agent loads: the real file pills over the panel skeleton for that file. */
export function AgentInstructionsSkeleton({ file, onFileChange }: AgentInstructionsSkeletonProps) {
  return (
    <div className="flex flex-col gap-3.5" data-testid="agent-instructions-skeleton">
      <InstructionsFileRow file={file} onFileChange={onFileChange}>
        {file === "agent" ? <Skeleton className="h-button-sm w-12" /> : null}
      </InstructionsFileRow>
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

const noop = () => undefined;

/**
 * The tab for a page that owns one status region around everything it loads and shows before its
 * code has loaded: the same frame, with the file pills out of reach and every panel a placeholder.
 */
export function AgentInstructionsPlaceholder({ file }: { file: AgentInstructionFile }) {
  return (
    <div className="flex flex-col gap-3.5" data-testid="agent-instructions-skeleton">
      <div className="contents" inert>
        <InstructionsFileRow file={file} onFileChange={noop}>
          {file === "agent" ? <Skeleton className="h-button-sm w-12" /> : null}
        </InstructionsFileRow>
      </div>
      {file === "agent" ? (
        <div aria-hidden="true">
          <Panel bodyClassName="p-0">
            <SkeletonText className="px-5 py-4 text-body" lines={6} />
          </Panel>
        </div>
      ) : (
        <div className="flex flex-col gap-4">
          {file === "heartbeat" ? <AgentHeartbeatOpsPlaceholder /> : null}
          <AuthoredFilePanelPlaceholder />
        </div>
      )}
    </div>
  );
}
