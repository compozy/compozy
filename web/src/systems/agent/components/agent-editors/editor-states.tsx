/** What every editor shows around its form: loading, a failed read, and who owns the file. */
import { Info, TriangleAlert } from "@compozy/ui/icons";

import { Alert, AlertDescription, Button, Empty, Skeleton, SkeletonRegion } from "@compozy/ui";

import { fileBlockCopy, type FileBlock } from "../../lib/agent-frontmatter";
import { formatAgentOwner } from "../../lib/agent-ownership";
import type { AgentPayload } from "../../types";

export function EditorLoading({ label }: { label: string }) {
  return (
    <SkeletonRegion className="flex flex-col gap-6 py-2" label={label}>
      <Skeleton className="h-14 w-72" />
      <Skeleton className="h-96 w-full" />
    </SkeletonRegion>
  );
}

export function EditorLoadError({ title, onRetry }: { title: string; onRetry: () => void }) {
  return (
    <Empty
      action={
        <Button onClick={onRetry} type="button" variant="secondary">
          Try again
        </Button>
      }
      fill={false}
      icon={TriangleAlert}
      title={title}
    />
  );
}

export function ReadOnlyNotice({ agent }: { agent: AgentPayload }) {
  return (
    <Alert variant="info">
      <Info aria-hidden="true" className="size-4" />
      <AlertDescription>
        {`${formatAgentOwner(agent)} owns this agent, so you can read this page but not change it.`}
      </AlertDescription>
    </Alert>
  );
}

export function FileBlockNotice({ block }: { block: FileBlock }) {
  return (
    <Alert variant="info">
      <Info aria-hidden="true" className="size-4" />
      <AlertDescription>{fileBlockCopy(block)}</AlertDescription>
    </Alert>
  );
}
