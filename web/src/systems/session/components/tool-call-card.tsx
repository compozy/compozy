import { CopyIconButton, ToolCallRow, type ToolCallStatus } from "@compozy/ui";
import { Suspense, useState, lazy } from "react";

import { compactSessionSummary } from "../lib/session-summary";
import { DetailPayload } from "./tool-renderers/detail-payload";

import { deriveToolRowStatus, hasToolInput, toolResultIsEmpty } from "../lib/message-parts";
import { isDeliberateTerminalTool, readSupervisedTerminalId } from "../lib/session-terminal-tools";
import { fileDiffStatForTool, type ToolFileDiffStat } from "../lib/tool-diff-stat";
import { resolveToolDisplay, type ToolDisplay } from "../lib/tool-display";
import {
  getToolLabel,
  getToolSettledLabel,
  resolveRegisteredToolName,
  toolHeadingName,
} from "../lib/tool-labels";
import type { UIMessage } from "../types";
import { isToolBodyField, type SessionToolBodyField } from "../lib/tool-matched-field";
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
  /** The row sits on a work group's rail: the rail's branch is its bullet, so no kind glyph. */
  onRail?: boolean;
}

/** Tools with specialized expanded renderers own input+output — no card-level JSON. */
const SPECIALIZED_TOOLS = new Set(["Bash", "Read", "Write", "Edit", "Grep", "Glob", "TodoWrite"]);

/** Serializes original tool identity, title, input, and output for detail copying. */
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

// The verb keeps its tense on failure — "Ran", never "Failed to run"; the ×
// glyph plus the error-first-line preview carry the failure. Only a family whose copy
// names the attempt ("Tried to …", subagent tools) differs.
function rowDisplay(message: UIMessage, status: ToolCallStatus): ToolDisplay {
  const live = status === "pending" || status === "running";
  const display = resolveToolDisplay(
    {
      toolName: message.toolName ?? "tool",
      ...(message.toolTitle ? { toolTitle: message.toolTitle } : {}),
      ...(message.toolInput ? { args: message.toolInput } : {}),
    },
    live ? "active" : "past"
  );
  if (status !== "failed" && status !== "absorbed") return display;
  const registryTool = resolveRegisteredToolName(message.toolName ?? "tool");
  const tried = getToolSettledLabel(registryTool, true);
  return tried === getToolLabel(registryTool, "past") ? display : { ...display, verb: tried };
}

function firstLine(text: string): string | undefined {
  for (const line of text.split(/\r?\n/)) {
    const trimmed = line.trim();
    if (trimmed.length > 0) return trimmed;
  }
  return undefined;
}

// A failed row's object is the error itself: the × glyph plus the error's
// first line carry the failure (ADR-009); Bash stderr keeps only its lead line.
function failurePreview(message: UIMessage, registryTool: string): string {
  const error = message.toolResult?.error;
  if (typeof error === "string" && error.trim().length > 0) {
    return compactSessionSummary(firstLine(error) ?? error);
  }
  const stderr = message.toolResult?.stderr;
  if (typeof stderr === "string" && stderr.trim().length > 0) {
    return compactSessionSummary(registryTool === "Bash" ? (firstLine(stderr) ?? stderr) : stderr);
  }
  return "Tool call failed";
}

function failureText(message: UIMessage): string {
  const error = message.toolResult?.error;
  if (typeof error === "string" && error.trim().length > 0) return error;
  const stderr = message.toolResult?.stderr;
  if (typeof stderr === "string" && stderr.trim().length > 0) return stderr;
  return "Tool call failed";
}

function diffStatLabel(additions: number, deletions: number): string {
  return `${additions} ${additions === 1 ? "addition" : "additions"}, ${deletions} ${deletions === 1 ? "deletion" : "deletions"}`;
}

/** Derives compact row presentation while retaining the complete inspection payload. */
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
  const display = rowDisplay(message, status);
  const failed = status === "failed" || status === "absorbed";
  const preview = failed ? failurePreview(message, registryTool) : display.target;
  // A compacted object keeps its full text one hover away.
  const previewTitle = failed ? preview : (display.fullTarget ?? display.target);
  const previewVariant: "chip" | "text" = failed || display.targetKind === "text" ? "text" : "chip";
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
    display,
    preview,
    previewTitle,
    previewVariant,
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

type ToolCallDiffStat = ToolFileDiffStat | null;

/** Accessible stat text: the state word, else the spelled-out diff stat. */
function toolCallStatLabel(
  stateWord: string | null,
  diffStat: ToolCallDiffStat
): string | undefined {
  if (stateWord) return stateWord;
  return diffStat ? diffStatLabel(diffStat.additions, diffStat.deletions) : undefined;
}

/** The row's trailing stat: a state word the glyph cannot carry, else the `+a −d` diff stat. */
function ToolCallStat({
  stateWord,
  diffStat,
}: {
  stateWord: string | null;
  diffStat: ToolCallDiffStat;
}) {
  if (stateWord) {
    return (
      <span className="text-subtle" data-testid="tool-call-state-word">
        {stateWord}
      </span>
    );
  }
  if (!diffStat) return null;
  return (
    <>
      {/* Per-call stats stay neutral; the sign carries the meaning. The turn's
          changed-files row is the one place additions/deletions take color. */}
      <span className="font-medium text-subtle">+{diffStat.additions}</span>
      <span className="font-medium text-subtle">−{diffStat.deletions}</span>
    </>
  );
}

/** The expanded body: the original title, the matched field in full, then the tool's own display. */
function ToolCallBody({
  message,
  titleDetail,
  matchedField,
  showArtifactResult,
}: {
  message: UIMessage;
  titleDetail?: string;
  matchedField: SessionToolBodyField | null;
  showArtifactResult: boolean;
}) {
  return (
    <>
      {titleDetail ? (
        <DetailPayload
          aria-label="Tool title"
          text={titleDetail}
          defaultExpanded
          downloadName="tool-title.txt"
        />
      ) : null}
      {matchedField ? <MatchedToolFieldContent message={message} field={matchedField} /> : null}
      {showArtifactResult && message.toolResult ? (
        <ToolResultArtifact result={message.toolResult} />
      ) : (
        <ExpandedToolContent message={message} />
      )}
    </>
  );
}

/**
 * Chat-thread tool surface composing `<ToolCallRow>` from `@compozy/ui`: one
 * calm 24px line — verb, then its object as a mono chip ("Read
 * `session-thread.tsx`", "Ran `go test ./...`") — whose status lives in the
 * trailing glyph. Failed rows stay collapsed — failure reads from the × glyph
 * and the error-first-line preview;
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
  onRail = false,
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
    display,
    preview,
    previewTitle,
    previewVariant,
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
  const statLabel = toolCallStatLabel(stateWord, diffStat);
  const copyAction = (
    <CopyIconButton
      value={copyPayload}
      copyLabel="Copy tool details"
      copiedLabel="Tool details copied"
      copyFailedLabel="Couldn't copy tool details"
      className="text-subtle hover:text-fg"
    />
  );

  return (
    <div data-testid="tool-call-row" data-part-index={partIndex}>
      <ToolCallRow
        toolName={display.verb}
        icon={onRail ? null : display.icon}
        preview={preview ? <span title={previewTitle}>{preview}</span> : undefined}
        previewVariant={previewVariant}
        status={status}
        errorMessage={errorMessage}
        actions={copyAction}
        expanded={ownExpanded || revealOpen}
        onExpandedChange={next => {
          if (!next && revealOpen) onRevealRelease?.();
          setOwnExpanded(next);
        }}
        statLabel={statLabel}
        stat={statLabel ? <ToolCallStat stateWord={stateWord} diffStat={diffStat} /> : undefined}
      >
        {showExpandedBody ? (
          <ToolCallBody
            message={message}
            titleDetail={showTitleDetail ? titleDetail : undefined}
            matchedField={matchedField}
            showArtifactResult={showArtifactResult}
          />
        ) : null}
      </ToolCallRow>
    </div>
  );
}
