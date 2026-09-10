import { CopyIconButton, ToolCallRow, type ToolCallStatus } from "@compozy/ui";
import { Suspense, useState, lazy } from "react";

import { compactSessionSummary } from "../lib/session-summary";
import { DetailPayload } from "./tool-renderers/detail-payload";

import { deriveToolRowStatus, hasToolInput, toolResultIsEmpty } from "../lib/message-parts";
import { isDeliberateTerminalTool, readSupervisedTerminalId } from "../lib/session-terminal-tools";
import { fileDiffStatForTool } from "../lib/tool-diff-stat";
import {
  getToolCompactSummary,
  getToolIcon,
  getToolLabel,
  resolveRegisteredToolName,
  toolHeadingName,
} from "../lib/tool-labels";
import type { UIMessage } from "../types";
import { isToolBodyField } from "../lib/tool-matched-field";
import { ExpandedToolContent } from "./tool-renderers/expanded-tool-content";
import { MatchedToolFieldContent } from "./tool-renderers/matched-field-content";
import { ToolResultArtifact } from "./tool-result-artifact";

const TerminalContent = lazy(async () => {
  const { TerminalContent: Content } = await import("./tool-renderers/terminal-content");
  return { default: Content };
});

export interface SessionToolCallRowProps {
  message: UIMessage;
  defaultExpanded?: boolean;
  /** The projected part this row renders (`data-part-index`), so find can land on it. */
  partIndex?: number;
  /** A find jump needs this body open; layered over the reader's own toggle. */
  revealOpen?: boolean;
  /** The field the daemon matched (`input | output | error | …`): its full payload is shown in the body. */
  revealField?: string;
  /** The reader closed a body a jump held open: the hold is theirs to drop. */
  onRevealRelease?: () => void;
  /**
   * True once the owning turn has settled. Neutral (empty-output) tools show
   * `empty` (Minus) while the turn streams and promote to `success` (Check) only
   * after it settles. Defaults to `false` (assume mid-stream unless told).
   */
  turnSettled?: boolean;
  /** The call was still running when the operator stopped the turn: reads "stopped", no glyph. */
  interrupted?: boolean;
  /**
   * The owning turn ended in a turn-level failure. Only then does a failed call
   * earn the danger glyph; otherwise a failure the turn absorbed reads as a
   * subtle × plus the word "failed" (ADR-009).
   */
  turnFailed?: boolean;
}

/** Tools with specialized expanded renderers own input+output — no card-level JSON. */
const SPECIALIZED_TOOLS = new Set(["Bash", "Read", "Write", "Edit", "Grep", "Glob", "TodoWrite"]);

function formatToolPayload(message: UIMessage): string {
  try {
    return JSON.stringify(
      {
        tool: message.toolName,
        ...(message.toolTitle ? { title: message.toolTitle } : {}),
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
    return compactSessionSummary(failurePreview(message, registryTool));
  }
  const description =
    message.toolTitle && message.toolTitle !== registryTool
      ? message.toolTitle
      : message.toolName !== toolHeadingName(registryTool)
        ? message.toolName
        : undefined;
  const summary = description
    ? compactSessionSummary(description)
    : getToolCompactSummary(registryTool, message.toolInput);
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

function toolCallPresentation({
  message,
  turnSettled,
  interrupted,
  turnFailed,
  revealOpen,
  revealField,
}: Required<
  Pick<
    SessionToolCallRowProps,
    "message" | "turnSettled" | "interrupted" | "turnFailed" | "revealOpen"
  >
> &
  Pick<SessionToolCallRowProps, "revealField">) {
  const derived = deriveToolRowStatus({
    toolError: message.toolError,
    toolResult: message.toolResult,
    hasInput: hasToolInput(message.toolInput),
    turnSettled,
  });
  const status: ToolCallStatus = interrupted
    ? "stopped"
    : derived.status === "failed" && !turnFailed
      ? "absorbed"
      : derived.status;
  const registryTool = resolveRegisteredToolName(message.toolName ?? "tool");
  const progressLabel = progressLabelFor(registryTool, status);
  const preview = previewFor(message, registryTool, status === "absorbed" ? "failed" : status);
  const toolIcon = getToolIcon(registryTool, message.toolInput);
  const copyPayload = formatToolPayload(message);
  const hasOutput = !toolResultIsEmpty(message.toolResult);
  const isSpecialized = SPECIALIZED_TOOLS.has(registryTool);
  const errorMessage =
    status === "failed" || status === "absorbed" ? failureText(message) : undefined;
  // The word that carries a state the glyph does not: "failed" beside the
  // subtle × of an absorbed failure, "stopped" on the call the operator cut.
  const stateWord = status === "absorbed" ? "failed" : status === "stopped" ? "stopped" : null;
  const diffStat =
    status === "success" && message.toolInput
      ? fileDiffStatForTool(registryTool, message.toolInput)
      : null;
  const showArtifactResult = message.toolResult?.truncated === true;
  // A find jump names the field it matched: that payload renders in full beside
  // the tool's own display, so the searched text is on screen (not only the
  // specialized summary of it). Title/name matches open the original title below.
  const matchedField = revealOpen && isToolBodyField(revealField) ? revealField : null;
  const titleDetail =
    revealField === "tool_name" ? message.toolName : (message.toolTitle ?? message.toolName);
  const showTitleDetail = Boolean(
    titleDetail &&
    (titleDetail !== toolHeadingName(registryTool) ||
      (revealOpen && (revealField === "title" || revealField === "tool_name")))
  );
  const showExpandedBody =
    showTitleDetail ||
    showArtifactResult ||
    isSpecialized ||
    hasOutput ||
    hasToolInput(message.toolInput) ||
    matchedField !== null;

  return {
    progressLabel,
    preview,
    toolIcon,
    copyPayload,
    status,
    errorMessage,
    stateWord,
    diffStat,
    matchedField,
    showArtifactResult,
    showExpandedBody,
    showTitleDetail,
    titleDetail,
  };
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
  partIndex,
  revealOpen = false,
  revealField,
  onRevealRelease,
  turnSettled = false,
  interrupted = false,
  turnFailed = false,
}: SessionToolCallRowProps) {
  const [ownExpanded, setOwnExpanded] = useState(defaultExpanded);
  if (
    isDeliberateTerminalTool(message.toolName) &&
    readSupervisedTerminalId(message.toolResult?.rawOutput)
  ) {
    return (
      <Suspense fallback={null}>
        <TerminalContent message={message} />
      </Suspense>
    );
  }
  const {
    progressLabel,
    preview,
    toolIcon,
    copyPayload,
    status,
    errorMessage,
    stateWord,
    diffStat,
    matchedField,
    showArtifactResult,
    showExpandedBody,
    showTitleDetail,
    titleDetail,
  } = toolCallPresentation({
    message,
    turnSettled,
    interrupted,
    turnFailed,
    revealOpen,
    revealField,
  });
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
    <div data-testid="tool-call-row" data-part-index={partIndex}>
      <ToolCallRow
        toolName={progressLabel}
        icon={toolIcon}
        preview={preview}
        status={status}
        errorMessage={errorMessage}
        actions={copyAction}
        expanded={ownExpanded || revealOpen}
        onExpandedChange={next => {
          if (!next && revealOpen) onRevealRelease?.();
          setOwnExpanded(next);
        }}
        statLabel={
          stateWord ??
          (diffStat ? diffStatLabel(diffStat.additions, diffStat.deletions) : undefined)
        }
        stat={
          stateWord ? (
            <span className="text-subtle" data-testid="tool-call-state-word">
              {stateWord}
            </span>
          ) : diffStat ? (
            <>
              <span className="font-medium text-success">+{diffStat.additions}</span>
              <span className="font-medium text-danger">−{diffStat.deletions}</span>
            </>
          ) : undefined
        }
      >
        {showExpandedBody ? (
          <ToolCallRow.Output>
            {showTitleDetail && titleDetail ? (
              <DetailPayload
                aria-label="Tool title"
                text={titleDetail}
                defaultExpanded
                downloadName="tool-title.txt"
              />
            ) : null}
            {matchedField ? (
              <MatchedToolFieldContent message={message} field={matchedField} />
            ) : null}
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
