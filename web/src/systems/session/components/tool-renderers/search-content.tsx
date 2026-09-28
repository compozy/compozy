import { searchToolView, shortenSearchPath } from "../../lib/tool-search-view";
import type { UIMessage } from "../../types";

const VISIBLE_RESULT_LINES = 20;

/** The first result lines as bare mono paths, with a count of the rest. */
function SearchMatchLines({ lines }: { lines: string[] }) {
  return (
    <div className="flex min-w-0 flex-col gap-px font-mono text-transcript-caption text-subtle">
      {lines.slice(0, VISIBLE_RESULT_LINES).map(line => (
        <span key={line} className="truncate" title={line}>
          {shortenSearchPath(line)}
        </span>
      ))}
      {lines.length > VISIBLE_RESULT_LINES ? (
        <span className="text-muted">+{lines.length - VISIBLE_RESULT_LINES} more</span>
      ) : null}
    </div>
  );
}

/**
 * Search detail as bare mono lines in the rail — the pattern as a context line,
 * matches as plain subtle text. No bordered list, no per-line icons.
 */
export function SearchContent({ message }: { message: UIMessage }) {
  const { pattern, scope, lines, hasResult } = searchToolView(message);

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="search-content">
      {pattern ? (
        <div className="font-mono text-transcript-caption text-subtle">
          {pattern}
          {scope ? <span className="ms-1.5 text-muted">in {scope}</span> : null}
        </div>
      ) : null}
      {lines.length > 0 ? (
        <SearchMatchLines lines={lines} />
      ) : hasResult ? (
        <span className="text-transcript-caption text-muted italic">No matches</span>
      ) : null}
    </div>
  );
}
