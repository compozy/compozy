import { useState } from "react";

import { cn } from "@/lib/utils";
import { ToolArtifactApiError } from "../adapters/tool-artifact-api";
import { useSessionRuntimeRenderContext } from "../hooks/use-session-runtime-render-context";
import { useToolArtifact } from "../hooks/use-tool-artifact";
import type { ToolArtifactRef, ToolUseResult } from "../types";
import { DetailPre } from "./tool-renderers/detail-pre";

const TOOL_ARTIFACT_URI_PREFIX = "compozy://tool-artifacts/";
const BYTE_FORMATTER = new Intl.NumberFormat();

function retainedArtifact(result: ToolUseResult): ToolArtifactRef | undefined {
  return result.artifacts?.find(artifact => artifact.uri.startsWith(TOOL_ARTIFACT_URI_PREFIX));
}

function byteProgress(loaded: number, total: number): string {
  return `${BYTE_FORMATTER.format(loaded)} of ${BYTE_FORMATTER.format(total)} bytes`;
}

/** Quiet `--info` text action in the artifact line — never a filled button. */
function ArtifactAction({
  label,
  disabled,
  onClick,
  testId,
  className,
}: {
  label: string;
  disabled?: boolean;
  onClick: () => void;
  testId?: string;
  className?: string;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      disabled={disabled}
      onClick={onClick}
      className={cn(
        "w-fit rounded-xs px-0.5 text-transcript-caption font-medium text-info transition-colors hover:underline hover:underline-offset-2 disabled:pointer-events-none disabled:opacity-60",
        className
      )}
    >
      {label}
    </button>
  );
}

/**
 * Retained-result affordance inside the tool detail rail (`.trow__artifact`):
 * the truncated preview stays in the mono pre; below it a quiet info action
 * loads the full artifact paginated **in place**, with faint mono byte
 * progress. Load failure is a danger text line with Retry — never an Alert.
 */
export function ToolResultArtifact({ result }: { result: ToolUseResult }) {
  const context = useSessionRuntimeRenderContext();
  const artifact = retainedArtifact(result);
  const [opened, setOpened] = useState(false);
  const artifactURI = artifact?.uri ?? "";
  const query = useToolArtifact(context?.workspaceId ?? "", artifactURI, opened && !!artifact);
  const canOpen = artifact !== undefined && context !== null;
  const error = query.error instanceof ToolArtifactApiError ? query.error : null;
  const showFull = opened && query.isSuccess;

  return (
    <div data-testid="tool-result-artifact" className="flex min-w-0 flex-col gap-1">
      {showFull ? (
        <DetailPre data-testid="full-tool-result">{query.content}</DetailPre>
      ) : result.preview ? (
        <DetailPre data-testid="tool-result-preview">{result.preview}</DetailPre>
      ) : null}

      <div className="flex min-w-0 flex-wrap items-baseline gap-2">
        {!opened && canOpen ? (
          <ArtifactAction
            label="Open full result"
            onClick={() => setOpened(true)}
            testId="artifact-open"
          />
        ) : null}
        {opened && query.isPending ? (
          <span className="text-transcript-caption text-subtle">Opening full result…</span>
        ) : null}
        {showFull && query.hasNextPage ? (
          <ArtifactAction
            label={query.isFetchingNextPage ? "Loading more…" : "Load more"}
            disabled={query.isFetchingNextPage}
            onClick={() => query.fetchNextPage()}
            testId="artifact-load-more"
          />
        ) : null}
        {showFull ? (
          <span className="font-mono text-badge text-faint tabular-nums">
            {byteProgress(query.loadedBytes, query.totalBytes)}
          </span>
        ) : !opened && canOpen && typeof artifact?.bytes === "number" ? (
          <span className="font-mono text-badge text-faint tabular-nums">
            {BYTE_FORMATTER.format(artifact.bytes)} bytes retained
          </span>
        ) : null}
      </div>

      {opened && query.isError ? (
        <div data-testid="artifact-error" className="text-transcript-caption text-danger">
          {error?.message ?? "Couldn't load the retained result. Try again."}
          {error?.statusCode !== 404 ? (
            <ArtifactAction
              label="Retry"
              onClick={() => query.refetch()}
              testId="artifact-retry"
              className="ml-1.5 px-0"
            />
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
