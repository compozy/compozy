import { Alert, AlertDescription, AlertTitle, Button, CodeBlock, Spinner } from "@agh/ui";
import { AlertTriangle } from "lucide-react";
import { useState } from "react";

import { ToolArtifactApiError } from "../adapters/tool-artifact-api";
import { useSessionRuntimeRenderContext } from "../hooks/use-session-runtime-render-context";
import { useToolArtifact } from "../hooks/use-tool-artifact";
import type { ToolArtifactRef, ToolUseResult } from "../types";

const TOOL_ARTIFACT_URI_PREFIX = "agh://tool-artifacts/";
const BYTE_FORMATTER = new Intl.NumberFormat();

function retainedArtifact(result: ToolUseResult): ToolArtifactRef | undefined {
  return result.artifacts?.find(artifact => artifact.uri.startsWith(TOOL_ARTIFACT_URI_PREFIX));
}

function byteProgress(loaded: number, total: number): string {
  return `${BYTE_FORMATTER.format(loaded)} of ${BYTE_FORMATTER.format(total)} bytes`;
}

export function ToolResultArtifact({ result }: { result: ToolUseResult }) {
  const context = useSessionRuntimeRenderContext();
  const artifact = retainedArtifact(result);
  const [opened, setOpened] = useState(false);
  const artifactURI = artifact?.uri ?? "";
  const query = useToolArtifact(context?.workspaceId ?? "", artifactURI, opened && !!artifact);
  const canOpen = artifact !== undefined && context !== null;
  const error = query.error instanceof ToolArtifactApiError ? query.error : null;

  return (
    <div data-testid="tool-result-artifact" className="flex min-w-0 flex-col gap-2.5">
      {result.preview ? (
        <CodeBlock
          caption="Result preview"
          code={result.preview}
          copyable
          density="compact"
          showPrompt={false}
          wrapLines
        />
      ) : null}

      {!opened && canOpen ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="w-fit"
          onClick={() => setOpened(true)}
        >
          Open full result
        </Button>
      ) : null}

      {opened && query.isPending ? (
        <Button type="button" variant="outline" size="sm" className="w-fit" disabled>
          <Spinner aria-hidden="true" className="size-3.5" />
          Opening full result
        </Button>
      ) : null}

      {opened && query.isError ? (
        <Alert variant="danger">
          <AlertTriangle aria-hidden="true" />
          <AlertTitle>Full result unavailable</AlertTitle>
          <AlertDescription>
            {error?.message ?? "Couldn't load the retained result. Try again."}
          </AlertDescription>
          {error?.statusCode !== 404 ? (
            <div className="col-start-2">
              <Button type="button" variant="outline" size="sm" onClick={() => query.refetch()}>
                Retry full result
              </Button>
            </div>
          ) : null}
        </Alert>
      ) : null}

      {opened && query.isSuccess ? (
        <div className="flex min-w-0 flex-col gap-2">
          <CodeBlock
            data-testid="full-tool-result"
            caption="Full tool result"
            code={query.content}
            copyable
            density="compact"
            language="json"
            showPrompt={false}
            wrapLines
          />
          <div className="flex min-w-0 flex-wrap items-center justify-between gap-2">
            <span className="text-form-hint text-faint tabular-nums">
              {byteProgress(query.loadedBytes, query.totalBytes)}
            </span>
            {query.hasNextPage ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={query.isFetchingNextPage}
                onClick={() => query.fetchNextPage()}
              >
                {query.isFetchingNextPage ? "Loading more result" : "Load more"}
              </Button>
            ) : null}
          </div>
        </div>
      ) : null}
    </div>
  );
}
