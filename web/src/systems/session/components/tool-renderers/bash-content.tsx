import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import type { UIMessage } from "../../types";
import { DetailPayload } from "./detail-payload";
import { DetailPre } from "./detail-pre";

const VISIBLE_OUTPUT_LINES = 20;

function clampLines(text: string, expanded: boolean, visibleLines = VISIBLE_OUTPUT_LINES): string {
  if (expanded) return text;
  if (visibleLines <= 0) return "";
  const lines = text.split("\n");
  if (lines.length <= visibleLines) return text;
  return lines.slice(0, visibleLines).join("\n");
}

function lineCount(text: string): number {
  return text.length > 0 ? text.split("\n").length : 0;
}

/** Format non-stderr output (stderr renders separately as danger text lines). */
function formatBashOutput(result: NonNullable<UIMessage["toolResult"]>): string {
  const parts: string[] = [];
  if (result.stdout) parts.push(result.stdout);
  if (result.content && !result.stdout) parts.push(result.content);
  return parts.join("\n");
}

/**
 * Bash detail in the rail grammar: the full command as a context line, stdout
 * as muted mono, stderr and error channels as danger **text** — never a tinted
 * block or ring.
 */
export function BashContent({ message }: { message: UIMessage }) {
  const command = message.toolInput?.command;
  const result = message.toolResult;
  const [expanded, setExpanded] = useState(false);

  const output = result ? formatBashOutput(result) : "";
  const stderr = result?.stderr ?? "";
  const errorText = result?.error ?? "";
  // Plain stdout is the payload that grows without bound (a verbose test run):
  // it renders through the bounded payload body with the truncation strip.
  if (output && !stderr && !errorText) {
    return (
      <div className="flex min-h-0 min-w-0 flex-col gap-1" data-testid="bash-content">
        {command ? (
          <DetailPre className="shrink-0 text-subtle" data-testid="bash-command">
            $ {String(command)}
          </DetailPre>
        ) : null}
        <DetailPayload downloadName="command-output.txt" text={output} />
      </div>
    );
  }

  return (
    <MixedBashOutput
      command={command}
      output={output}
      stderr={stderr}
      errorText={errorText}
      expanded={expanded}
      onToggle={() => setExpanded(value => !value)}
    />
  );
}

function MixedBashOutput({
  command,
  output,
  stderr,
  errorText,
  expanded,
  onToggle,
}: {
  command: unknown;
  output: string;
  stderr: string;
  errorText: string;
  expanded: boolean;
  onToggle: () => void;
}) {
  const outputLines = lineCount(output);
  const stderrLines = lineCount(stderr);
  const errorLines = lineCount(errorText);
  const totalLines = outputLines + stderrLines + errorLines;
  const overflow = totalLines > VISIBLE_OUTPUT_LINES;
  let remainingLines = VISIBLE_OUTPUT_LINES;
  const visibleOutput = clampLines(output, expanded, remainingLines);
  remainingLines = Math.max(0, remainingLines - outputLines);
  const visibleStderr = clampLines(stderr, expanded, remainingLines);
  remainingLines = Math.max(0, remainingLines - stderrLines);
  const visibleError = clampLines(errorText, expanded, remainingLines);

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="bash-content">
      {command ? (
        <DetailPre className="text-subtle" data-testid="bash-command">
          $ {String(command)}
        </DetailPre>
      ) : null}
      <BashOutputLines output={visibleOutput} stderr={visibleStderr} errorText={visibleError} />
      {overflow ? (
        <button
          type="button"
          onClick={onToggle}
          className="flex w-fit items-center gap-1 text-[11.5px] text-subtle transition-colors hover:text-fg"
        >
          <ChevronsUpDown aria-hidden="true" className="size-3" />
          {expanded ? "Collapse" : `Show full output (${totalLines} lines)`}
        </button>
      ) : null}
    </div>
  );
}

function BashOutputLines({
  output,
  stderr,
  errorText,
}: {
  output: string;
  stderr: string;
  errorText: string;
}) {
  return output || stderr || errorText ? (
    <DetailPre>
      {output || null}
      {output && (stderr || errorText) ? "\n" : null}
      {stderr ? (
        <span className="text-danger" data-testid="bash-stderr">
          {stderr}
        </span>
      ) : null}
      {stderr && errorText ? "\n" : null}
      {errorText ? <span className="text-danger">{errorText}</span> : null}
    </DetailPre>
  ) : null;
}
