import { markdownLineToPlainText } from "../../lib/markdown-plain-text";
import { collapseWhitespace, isSubagentLive } from "./subagent-format";
import type { SubagentView } from "./types";

const HOVER_PREVIEW_MAX = 280;

/** Hover preview text (UT-W08): plain text, whitespace collapsed, capped at 280 chars + `…`. */
export function subagentHoverPreview(subagent: SubagentView): string {
  const source = isSubagentLive(subagent.status)
    ? subagent.progress
    : subagent.status === "failed"
      ? subagent.error || subagent.result_preview
      : subagent.result_preview;
  const text = collapseWhitespace(
    (source ?? "").split(/\r?\n/).map(markdownLineToPlainText).join(" ")
  );
  return text.length > HOVER_PREVIEW_MAX ? `${text.slice(0, HOVER_PREVIEW_MAX).trimEnd()}…` : text;
}
