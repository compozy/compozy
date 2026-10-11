import { DiffCard, type DiffCardLine } from "@compozy/ui";
import { ChevronsUpDown } from "lucide-react";
import { useState } from "react";

import type { AlignedDiffLine } from "../../lib/tool-diff-stat";

/** Lines a collapsed diff shows before "Show full diff". */
const COLLAPSED_LINES = 12;

function lineId(line: AlignedDiffLine): string {
  return `${line.kind}:${line.oldIndex ?? ""}:${line.newIndex ?? ""}`;
}

/**
 * A file change as a diff card: path and colored `+a −d` header, the aligned
 * lines collapsed to the first dozen with a control that opens the rest.
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
  const visible = showFull ? lines : lines.slice(0, COLLAPSED_LINES);
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
