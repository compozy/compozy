import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import { editDiffView, readEditToolInput } from "../../lib/tool-edit-diff";
import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";
import { GenericContent } from "./generic-content";

/** Deletions then additions as colored text spans inside one detail block. */
function EditDiffLines({ removed, added }: { removed: string[]; added: string[] }) {
  if (removed.length === 0 && added.length === 0) return null;
  return (
    <DetailPre>
      {removed.length > 0 ? (
        <span className="text-danger" data-testid="edit-removed">
          {removed.join("\n")}
        </span>
      ) : null}
      {removed.length > 0 && added.length > 0 ? "\n" : null}
      {added.length > 0 ? (
        <span className="text-success" data-testid="edit-added">
          {added.join("\n")}
        </span>
      ) : null}
    </DetailPre>
  );
}

/**
 * Edit detail as a unified diff in colored **text** lines only — deletions
 * first as `- ` lines at `--danger`, additions as `+ ` lines at `--success` —
 * inside the plain detail rail. No stacked tinted blocks, no line-number chrome.
 */
export function EditContent({ message }: { message: UIMessage }) {
  const [showFull, setShowFull] = useState(false);
  const { filePath, oldStr, newStr } = readEditToolInput(message);

  if (!filePath && !oldStr && !newStr) {
    return <GenericContent message={message} />;
  }

  const { removed, added, truncated } = editDiffView(oldStr, newStr, showFull);

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="edit-content">
      {filePath ? (
        <div className="font-mono text-transcript-caption text-subtle">{filePath}</div>
      ) : null}
      <EditDiffLines removed={removed} added={added} />
      {truncated ? (
        <button
          type="button"
          onClick={() => setShowFull(true)}
          className="flex w-fit items-center gap-1 text-transcript-meta text-subtle transition-colors hover:text-fg"
        >
          <ChevronsUpDown aria-hidden="true" className="size-3" />
          Show full content
        </button>
      ) : null}
    </div>
  );
}
