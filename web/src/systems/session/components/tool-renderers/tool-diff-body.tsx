import { DiffCard, type DiffCardLine } from "@compozy/ui";
import { ChevronsUpDown } from "@compozy/ui/icons";
import { useState } from "react";

import type { AlignedDiffLine } from "../../lib/tool-diff-stat";

/** Lines a collapsed diff shows before "Show full diff". */
const COLLAPSED_LINES = 12;
/** Unchanged lines a collapsed diff keeps above its first change. */
const LEADING_CONTEXT = 3;

/** The collapsed window: it opens just above the first change so the change shows. */
function collapsedWindow(lines: readonly AlignedDiffLine[]): readonly AlignedDiffLine[] {
  const firstChange = lines.findIndex(line => line.kind !== "context");
  const start = Math.min(
    Math.max(0, firstChange - LEADING_CONTEXT),
    Math.max(0, lines.length - COLLAPSED_LINES)
  );
  return lines.slice(start, start + COLLAPSED_LINES);
}

function lineId(line: AlignedDiffLine): string {
  return `${line.kind}:${line.oldIndex ?? ""}:${line.newIndex ?? ""}`;
}

/**
 * A file change as a diff card: path and colored `+a −d` header, the aligned
 * lines collapsed to a dozen around the first change with a control that opens
 * the rest.
 */
export function ToolDiffBody({
  path,
  lines,
  testId,
}: {
  path?: string;
  lines: readonly AlignedDiffLine[];
  testId: string;
}) {
  const [showFull, setShowFull] = useState(false);
  const visible = showFull ? lines : collapsedWindow(lines);
  const hidden = lines.length - visible.length;
  const cardLines: DiffCardLine[] = visible.map(line => ({
    id: lineId(line),
    kind: line.kind,
    text: line.text,
  }));
  return (
    <DiffCard
      data-testid={testId}
      {...(path ? { path: <span title={path}>{path}</span> } : {})}
      additions={lines.filter(line => line.kind === "add").length}
      deletions={lines.filter(line => line.kind === "del").length}
      lines={cardLines}
      footer={
        hidden > 0 ? (
          <button
            type="button"
            onClick={() => setShowFull(true)}
            className="flex w-fit items-center gap-1 text-transcript-meta text-subtle transition-colors duration-base hover:text-fg"
          >
            <ChevronsUpDown aria-hidden="true" className="size-3" />
            Show full diff ({lines.length} lines)
          </button>
        ) : undefined
      }
    />
  );
}
