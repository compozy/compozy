import { FileText } from "lucide-react";

import type { UIMessage } from "../../types";

function shortenPath(filePath: string): string {
  const parts = filePath.split("/");
  if (parts.length <= 3) return filePath;
  return parts.slice(-3).join("/");
}

export function SearchContent({ message }: { message: UIMessage }) {
  const pattern = String(message.toolInput?.pattern ?? "");
  const glob = message.toolInput?.glob ? String(message.toolInput.glob) : "";
  const path = message.toolInput?.path ? String(message.toolInput.path) : "";
  const result = message.toolResult;

  const resultText = result?.stdout ?? result?.content ?? "";
  const lines = resultText ? resultText.split("\n").filter(Boolean) : [];

  return (
    <div className="space-y-1.5 text-small-body" data-testid="search-content">
      {pattern ? (
        <div className="font-mono text-form-label text-subtle">
          {pattern}
          {glob ? <span className="ms-1.5 text-muted">in {glob}</span> : null}
          {!glob && path ? <span className="ms-1.5 text-muted">in {shortenPath(path)}</span> : null}
        </div>
      ) : null}
      {lines.length > 0 ? (
        <div className="overflow-hidden rounded-md border border-line">
          {lines.slice(0, 20).map(line => (
            <div
              key={line}
              className="flex items-center gap-2 border-t border-line px-3 py-1 text-form-label first:border-t-0"
            >
              <FileText aria-hidden="true" className="size-3 shrink-0 text-subtle" />
              <span className="truncate font-mono text-subtle" title={line}>
                {shortenPath(line)}
              </span>
            </div>
          ))}
          {lines.length > 20 ? (
            <div className="border-t border-line px-3 py-1 text-badge text-muted">
              +{lines.length - 20} more
            </div>
          ) : null}
        </div>
      ) : result ? (
        <span className="text-badge text-muted italic">No matches</span>
      ) : null}
    </div>
  );
}
