import type { ReactNode } from "react";

import type { FilesSearchState } from "../hooks/use-files-search";
import type { FilesSearchOptions } from "../hooks/use-files-search-prefs";
import { activeFilters, searchNotes, searchSummary } from "../lib/search-summary";

function Line({
  children,
  testId,
  tone = "muted",
}: {
  children: ReactNode;
  testId: string;
  tone?: "muted" | "fg";
}) {
  return (
    <p
      data-testid={testId}
      className={tone === "fg" ? "text-xs text-fg-2" : "text-xs text-muted-foreground"}
    >
      {children}
    </p>
  );
}

function ProgressLine({ active }: { active: boolean }) {
  return (
    <div className="h-0.5 w-full overflow-hidden" aria-hidden>
      {active ? <div className="h-full w-1/3 animate-pulse rounded-pill bg-brand" /> : null}
    </div>
  );
}

/** Everything between the query and the result list for one search state. */
export function FilesSearchStatus({
  state,
  draft,
  options,
}: {
  state: FilesSearchState;
  draft: string;
  options: FilesSearchOptions;
}) {
  const busy = state.kind === "loading" || (state.kind === "results" && state.searching);
  let body: ReactNode = null;
  switch (state.kind) {
    case "idle":
      body = <Line testId="files-search-idle">Search text across this project</Line>;
      break;
    case "short":
      body = <Line testId="files-search-short">Type at least 2 characters</Line>;
      break;
    case "loading":
    case "invalid":
      break;
    case "unavailable":
      body = <Line testId="files-search-unavailable">This worktree is no longer on disk</Line>;
      break;
    case "error":
      body = <Line testId="files-search-failed">{state.message}</Line>;
      break;
    case "results": {
      const { response } = state;
      if (response.files.length === 0) {
        const filters = activeFilters(options);
        body = (
          <>
            <Line testId="files-search-empty" tone="fg">
              No results for <code className="font-mono">{draft}</code>
            </Line>
            {filters ? <Line testId="files-search-filters">{filters}</Line> : null}
          </>
        );
      } else {
        body = (
          <Line testId="files-search-summary" tone="fg">
            {searchSummary(response)}
          </Line>
        );
      }
      const notes = searchNotes(response);
      if (notes.length > 0) {
        body = (
          <>
            {body}
            {notes.map(note => (
              <Line key={note} testId="files-search-note">
                {note}
              </Line>
            ))}
          </>
        );
      }
      break;
    }
  }
  return (
    <div className="flex shrink-0 flex-col" role="status" aria-busy={busy}>
      <ProgressLine active={busy} />
      {body ? <div className="flex flex-col gap-0.5 px-3 py-1.5">{body}</div> : null}
    </div>
  );
}
