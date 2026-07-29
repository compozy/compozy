import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";
import { GenericContent } from "./generic-content";

const TRUNCATE_THRESHOLD = 1500;

function diffLines(source: string, marker: "-" | "+"): string[] {
  if (source.length === 0) return [];
  return source.split("\n").map(line => `${marker} ${line}`);
}

/**
 * Edit detail as a unified diff in colored **text** lines only — deletions
 * first as `- ` lines at `--danger`, additions as `+ ` lines at `--success` —
 * inside the plain detail rail. No stacked tinted blocks, no line-number chrome.
 */
export function EditContent({ message }: { message: UIMessage }) {
  const [showFull, setShowFull] = useState(false);
  const filePath = String(
    message.toolInput?.file_path ??
      message.toolInput?.filePath ??
      message.toolResult?.filePath ??
      ""
  );
  const rawOld = message.toolInput?.old_string;
  const rawNew = message.toolInput?.new_string;
  const oldStr = rawOld != null ? String(rawOld) : "";
  const newStr = rawNew != null ? String(rawNew) : "";
  const isTruncated =
    !showFull && (oldStr.length > TRUNCATE_THRESHOLD || newStr.length > TRUNCATE_THRESHOLD);

  if (!filePath && !oldStr && !newStr) {
    return <GenericContent message={message} />;
  }

  const displayOld = showFull ? oldStr : oldStr.slice(0, TRUNCATE_THRESHOLD);
  const displayNew = showFull ? newStr : newStr.slice(0, TRUNCATE_THRESHOLD);
  const oldSuffix = !showFull && oldStr.length > TRUNCATE_THRESHOLD ? "…" : "";
  const newSuffix = !showFull && newStr.length > TRUNCATE_THRESHOLD ? "…" : "";
  const removed = diffLines(`${displayOld}${oldSuffix}`, "-");
  const added = diffLines(`${displayNew}${newSuffix}`, "+");

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="edit-content">
      {filePath ? <div className="font-mono text-[11px] text-subtle">{filePath}</div> : null}
      {removed.length > 0 || added.length > 0 ? (
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
      ) : null}
      {isTruncated ? (
        <button
          type="button"
          onClick={() => setShowFull(true)}
          className="flex w-fit items-center gap-1 text-[11.5px] text-subtle transition-colors hover:text-fg"
        >
          <ChevronsUpDown aria-hidden="true" className="size-3" />
          Show full content
        </button>
      ) : null}
    </div>
  );
}
