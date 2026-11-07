"use client";

import { CodeView } from "@pierre/diffs/react";
import type * as React from "react";

import { InfoIcon } from "../../icons";
import { cn } from "../../lib/utils";
import { CodeViewerPlainText } from "./code-viewer-plain";
import { useCodeViewer } from "./hooks/use-code-viewer";
import type { CodeViewerMatch } from "./hooks/use-code-viewer-find";

export type { CodeViewerMatch } from "./hooks/use-code-viewer-find";

export interface CodeViewerProps extends Omit<React.ComponentProps<"div">, "children"> {
  /** Root-relative path; names the file for language inference. */
  path: string;
  text: string;
  /** Shiki language override; inferred from `path` when omitted. */
  language?: string;
  /** 1-based line to scroll into view and highlight. */
  targetLine?: number | null;
  /** Changes when the content changes, so the highlight cache stays correct. */
  cacheKey?: string;
  /** Header row above the code (path, size, actions). */
  header?: React.ReactNode;
  /** Inline notice between header and code, such as a truncation note. */
  notice?: React.ReactNode;
  /** Copy for a target past the end of the file. */
  pastEndNotice?: (requestedLine: number, lastLine: number) => React.ReactNode;
  wrap?: boolean;
  /** Find matches to paint over the text; the caller computes them from `text`. */
  findMatches?: readonly CodeViewerMatch[];
  /** Index into `findMatches` of the current match; it scrolls into view with stronger emphasis. */
  activeFindMatch?: number | null;
}

function defaultPastEndNotice(requestedLine: number, lastLine: number): React.ReactNode {
  return `Line ${requestedLine.toLocaleString("en-US")} is past the end of this file. Showing line ${lastLine.toLocaleString("en-US")}.`;
}

/**
 * Read-only, virtualized, highlighted single-file view with line numbers and a
 * highlighted target line. Domain states (loading, binary, sensitive, …) are
 * composed around it by the caller.
 */
function CodeViewer({
  path,
  text,
  language,
  targetLine,
  cacheKey,
  header,
  notice,
  pastEndNotice = defaultPastEndNotice,
  wrap = false,
  findMatches,
  activeFindMatch,
  className,
  ...props
}: CodeViewerProps) {
  const viewer = useCodeViewer({
    activeFindMatch,
    cacheKey,
    findMatches,
    language,
    path,
    targetLine,
    text,
    wrap,
  });
  const { handleRef, items, lineCount, options, painted, selectedLines, target } = viewer;

  return (
    <div
      data-slot="code-viewer"
      data-line-count={lineCount}
      data-highlighted-line={target?.line}
      data-find-matches={findMatches ? findMatches.length : undefined}
      data-find-active-line={viewer.activeFindLine ?? undefined}
      className={cn("flex h-full min-h-0 flex-col bg-canvas", className)}
      {...props}
    >
      {header}
      {target?.pastEnd ? (
        <p
          role="status"
          data-slot="code-viewer-past-end"
          className="flex items-center gap-2 border-b border-line-soft px-3 py-1.5 text-xs text-muted-foreground"
        >
          <InfoIcon aria-hidden className="size-3.5 shrink-0" />
          {pastEndNotice(target.requested, target.line)}
        </p>
      ) : null}
      {notice}
      <div className="relative flex min-h-0 flex-1 flex-col">
        <CodeView
          ref={handleRef}
          items={items}
          options={options}
          selectedLines={selectedLines}
          containerRef={viewer.containerRef}
          onScroll={viewer.onScroll}
          className="min-h-0 flex-1 overflow-auto"
        />
        {painted ? null : (
          <CodeViewerPlainText
            text={text}
            lineCount={lineCount}
            targetLine={target?.line ?? null}
          />
        )}
      </div>
    </div>
  );
}

export { CodeViewer };
