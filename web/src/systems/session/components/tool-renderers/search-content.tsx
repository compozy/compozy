import type { UIMessage } from "../../types";

const VISIBLE_RESULT_LINES = 20;

function shortenPath(filePath: string): string {
  const parts = filePath.split("/");
  if (parts.length <= 3) return filePath;
  return parts.slice(-3).join("/");
}

/**
 * Search detail as bare mono lines in the rail — the pattern as a context line,
 * matches as plain subtle text. No bordered list, no per-line icons.
 */
export function SearchContent({ message }: { message: UIMessage }) {
  const pattern = String(message.toolInput?.pattern ?? "");
  const glob = message.toolInput?.glob ? String(message.toolInput.glob) : "";
  const path = message.toolInput?.path ? String(message.toolInput.path) : "";
  const result = message.toolResult;

  const resultText = result?.stdout ?? result?.content ?? "";
  const lines = resultText ? resultText.split("\n").filter(Boolean) : [];

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="search-content">
      {pattern ? (
        <div className="font-mono text-[11px] text-subtle">
          {pattern}
          {glob ? <span className="ms-1.5 text-muted">in {glob}</span> : null}
          {!glob && path ? <span className="ms-1.5 text-muted">in {shortenPath(path)}</span> : null}
        </div>
      ) : null}
      {lines.length > 0 ? (
        <div className="flex min-w-0 flex-col gap-px font-mono text-[11px] text-subtle">
          {lines.slice(0, VISIBLE_RESULT_LINES).map(line => (
            <span key={line} className="truncate" title={line}>
              {shortenPath(line)}
            </span>
          ))}
          {lines.length > VISIBLE_RESULT_LINES ? (
            <span className="text-muted">+{lines.length - VISIBLE_RESULT_LINES} more</span>
          ) : null}
        </div>
      ) : result ? (
        <span className="text-[11px] text-muted italic">No matches</span>
      ) : null}
    </div>
  );
}
