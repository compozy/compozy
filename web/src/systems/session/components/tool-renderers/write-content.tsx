import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";
import { GenericContent } from "./generic-content";

const TRUNCATE_THRESHOLD = 2000;

/**
 * Write detail in the rail grammar: the target path as a context line and the
 * written content as added `+ ` lines at `--success` — colored text, no block.
 */
export function WriteContent({ message }: { message: UIMessage }) {
  const [showFull, setShowFull] = useState(false);
  const filePath = String(
    message.toolInput?.file_path ??
      message.toolInput?.filePath ??
      message.toolResult?.filePath ??
      ""
  );
  const content = String(message.toolInput?.content ?? message.toolResult?.content ?? "");
  const isTruncated = !showFull && content.length > TRUNCATE_THRESHOLD;
  const displayContent = showFull ? content : content.slice(0, TRUNCATE_THRESHOLD);

  if (!filePath && !content) {
    return <GenericContent message={message} />;
  }

  const added = `${displayContent}${isTruncated ? "…" : ""}`
    .split("\n")
    .map(line => `+ ${line}`)
    .join("\n");

  return (
    <div className="flex min-w-0 flex-col gap-1" data-testid="write-content">
      {filePath ? <div className="font-mono text-[11px] text-subtle">{filePath}</div> : null}
      {content ? (
        <DetailPre>
          <span className="text-success">{added}</span>
        </DetailPre>
      ) : null}
      {isTruncated ? (
        <button
          type="button"
          onClick={() => setShowFull(true)}
          className="flex w-fit items-center gap-1 text-[11.5px] text-subtle transition-colors hover:text-fg"
        >
          <ChevronsUpDown aria-hidden="true" className="size-3" />
          Show full content ({content.length.toLocaleString()} chars)
        </button>
      ) : null}
    </div>
  );
}
