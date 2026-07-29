import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";

const VISIBLE_OUTPUT_LINES = 20;

function clampLines(text: string, expanded: boolean): string {
  if (expanded) return text;
  const lines = text.split("\n");
  if (lines.length <= VISIBLE_OUTPUT_LINES) return text;
  return lines.slice(0, VISIBLE_OUTPUT_LINES).join("\n");
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
  const totalLines =
    (output ? output.split("\n").length : 0) + (stderr ? stderr.split("\n").length : 0);
  const overflow = totalLines > VISIBLE_OUTPUT_LINES;

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="bash-content">
      {command ? (
        <DetailPre className="text-subtle" data-testid="bash-command">
          $ {String(command)}
        </DetailPre>
      ) : null}
      {output || stderr || errorText ? (
        <DetailPre>
          {output ? clampLines(output, expanded) : null}
          {output && (stderr || errorText) ? "\n" : null}
          {stderr ? (
            <span className="text-danger" data-testid="bash-stderr">
              {clampLines(stderr, expanded)}
            </span>
          ) : null}
          {stderr && errorText ? "\n" : null}
          {errorText ? <span className="text-danger">{errorText}</span> : null}
        </DetailPre>
      ) : null}
      {overflow ? (
        <button
          type="button"
          onClick={() => setExpanded(value => !value)}
          className="flex w-fit items-center gap-1 text-[11.5px] text-subtle transition-colors hover:text-fg"
        >
          <ChevronsUpDown aria-hidden="true" className="size-3" />
          {expanded ? "Collapse" : `Show full output (${totalLines} lines)`}
        </button>
      ) : null}
    </div>
  );
}
