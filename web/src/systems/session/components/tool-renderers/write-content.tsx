import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import { CodeBlock } from "@agh/ui";

import type { UIMessage } from "../../types";
import { GenericContent } from "./generic-content";

const TRUNCATE_THRESHOLD = 2000;

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

  return (
    <div className="space-y-1.5 text-small-body" data-testid="write-content">
      {filePath ? <div className="font-mono text-form-label text-subtle">{filePath}</div> : null}
      {content ? (
        <CodeBlock
          code={`${displayContent}${isTruncated ? "\u2026" : ""}`}
          copyable={false}
          density="compact"
          showPrompt={false}
          truncateLines={16}
        />
      ) : null}
      {isTruncated ? (
        <button
          type="button"
          onClick={() => setShowFull(true)}
          className="flex items-center gap-1 text-form-label text-muted hover:text-fg transition-colors"
        >
          <ChevronsUpDown className="size-3" />
          Show full content ({content.length.toLocaleString()} chars)
        </button>
      ) : null}
    </div>
  );
}
