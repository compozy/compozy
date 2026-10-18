import type { UIMessage } from "../types";

export interface SearchToolView {
  pattern: string;
  /** Where the search ran: the glob, else the shortened path, else empty. */
  scope: string;
  /** Non-empty result lines (matched files or matches). */
  lines: string[];
  hasResult: boolean;
}

/** Keeps only the last three segments of a long path. */
export function shortenSearchPath(filePath: string): string {
  const parts = filePath.split("/");
  if (parts.length <= 3) return filePath;
  return parts.slice(-3).join("/");
}

/** Reads a Grep/Glob call into its pattern, scope, and result lines. */
export function searchToolView(message: UIMessage): SearchToolView {
  const input = message.toolInput;
  const glob = input?.glob ? String(input.glob) : "";
  const path = input?.path ? shortenSearchPath(String(input.path)) : "";
  const result = message.toolResult;
  const resultText = result?.stdout ?? result?.content ?? "";
  return {
    pattern: String(input?.pattern ?? ""),
    scope: glob || path,
    lines: resultText ? resultText.split("\n").filter(Boolean) : [],
    hasResult: Boolean(result),
  };
}
