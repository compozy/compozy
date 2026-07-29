import { CopyIconButton, ToolCallRow, type ToolCallStatus } from "@compozy/ui";

import { deriveToolRowStatus, hasToolInput, toolResultIsEmpty } from "../lib/message-parts";
import { fileDiffStatForTool } from "../lib/tool-diff-stat";
import {
  getToolCompactSummary,
  getToolIcon,
  getToolLabel,
  resolveRegisteredToolName,
} from "../lib/tool-labels";
import type { UIMessage } from "../types";
import { ExpandedToolContent } from "./tool-renderers/expanded-tool-content";
import { ToolResultArtifact } from "./tool-result-artifact";

export interface SessionToolCallRowProps {
  message: UIMessage;
  defaultExpanded?: boolean;
  /**
   * True once the owning turn has settled. Neutral (empty-output) tools show
   * `empty` (Minus) while the turn streams and promote to `success` (Check) only
   * after it settles. Defaults to `false` (assume mid-stream unless told).
   */
  turnSettled?: boolean;
}

/** Tools with specialized expanded renderers own input+output — no card-level JSON. */
const SPECIALIZED_TOOLS = new Set(["Bash", "Read", "Write", "Edit", "Grep", "Glob", "TodoWrite"]);

function formatToolPayload(message: UIMessage): string {
  try {
    return JSON.stringify(
      {
        tool: message.toolName,
        input: message.toolInput ?? {},
        output: message.toolResult ?? null,
        error: message.toolError === true,
      },
      null,
      2
    );
  } catch {
    return String(message.toolName ?? "tool");
  }
}

// The verb keeps its tense on failure — "Ran", never "Failed to run"; the
// danger × glyph plus the error-first-line preview carry the failure.
function progressLabelFor(toolName: string, status: ToolCallStatus): string {
  if (status === "pending" || status === "running") {
    return getToolLabel(toolName, "active");
  }
  return getToolLabel(toolName, "past");
}

function firstLine(text: string): string | undefined {
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return undefined;
}

function failureText(message: UIMessage): string {
  const error = message.toolResult?.error;
  if (typeof error === "string" && error.trim().length > 0) return error;
  const stderr = message.toolResult?.stderr;
  if (typeof stderr === "string" && stderr.trim().length > 0) return stderr;
  return "Tool call failed";
}

function failurePreview(message: UIMessage, registryTool: string): string {
  const error = message.toolResult?.error;
  if (typeof error === "string" && error.trim().length > 0) {
    return firstLine(error) ?? error;
  }
  const stderr = message.toolResult?.stderr;
  if (typeof stderr === "string" && stderr.trim().length > 0) {
    return registryTool === "Bash" ? (firstLine(stderr) ?? stderr) : stderr;
  }
  return "Tool call failed";
}

// Collapsed-row preview: failed rows lead with the error's first line; settled
// Reads append their line count; everything else keeps the compact input summary.
function previewFor(
  message: UIMessage,
  registryTool: string,
  status: ToolCallStatus
): string | undefined {
  if (status === "failed") {
    return failurePreview(message, registryTool);
  }
  const summary = getToolCompactSummary(registryTool, message.toolInput);
  if (registryTool === "Read" && status === "success" && summary) {
    const body = message.toolResult?.stdout ?? message.toolResult?.content;
    if (typeof body === "string" && body.length > 0) {
      const lineCount = body.split("\n").length;
      return `${summary} · ${lineCount} ${lineCount === 1 ? "line" : "lines"}`;
    }
  }
  return summary;
}

function diffStatLabel(additions: number, deletions: number): string {
  return `${additions} ${additions === 1 ? "addition" : "additions"}, ${deletions} ${deletions === 1 ? "deletion" : "deletions"}`;
}

/**
 * Chat-thread tool surface composing `<ToolCallRow>` from `@compozy/ui`: one
 * calm 24px line whose status lives in the trailing glyph. Failed rows stay
 * collapsed — failure reads from the × glyph and the error-first-line preview;
 * successful Edit/Write rows carry their per-file `+a −d` stat.
 */
export function SessionToolCallRow({
  message,
  defaultExpanded = false,
  turnSettled = false,
}: SessionToolCallRowProps) {
  const { status } = deriveToolRowStatus({
    toolError: message.toolError,
    toolResult: message.toolResult,
    hasInput: hasToolInput(message.toolInput),
    turnSettled,
  });
  const registryTool = resolveRegisteredToolName(message.toolName ?? "tool");
  const progressLabel = progressLabelFor(registryTool, status);
  const preview = previewFor(message, registryTool, status);
  const toolIcon = getToolIcon(registryTool, message.toolInput);
  const copyPayload = formatToolPayload(message);
  const hasOutput = !toolResultIsEmpty(message.toolResult);
  const isSpecialized = SPECIALIZED_TOOLS.has(registryTool);
  const errorMessage = status === "failed" ? failureText(message) : undefined;
  const diffStat =
    status === "success" && message.toolInput
      ? fileDiffStatForTool(registryTool, message.toolInput)
      : null;
  const showArtifactResult = message.toolResult?.truncated === true;
  const showExpandedBody =
    showArtifactResult || isSpecialized || hasOutput || hasToolInput(message.toolInput);
  const copyAction = (
    <CopyIconButton
      value={copyPayload}
      copyLabel="Copy tool payload"
      copiedLabel="Tool payload copied"
      copyFailedLabel="Tool payload copy failed"
      className="text-subtle hover:text-fg"
    />
  );

  return (
    <div data-testid="tool-call-row">
      <ToolCallRow
        toolName={progressLabel}
        icon={toolIcon}
        preview={preview}
        status={status}
        errorMessage={errorMessage}
        actions={copyAction}
        defaultExpanded={defaultExpanded}
        statLabel={diffStat ? diffStatLabel(diffStat.additions, diffStat.deletions) : undefined}
        stat={
          diffStat ? (
            <>
              <span className="font-medium text-success">+{diffStat.additions}</span>
              <span className="font-medium text-danger">−{diffStat.deletions}</span>
            </>
          ) : undefined
        }
      >
        {showExpandedBody ? (
          <ToolCallRow.Output>
            {showArtifactResult && message.toolResult ? (
              <ToolResultArtifact result={message.toolResult} />
            ) : (
              <ExpandedToolContent message={message} />
            )}
          </ToolCallRow.Output>
        ) : null}
      </ToolCallRow>
    </div>
  );
}
