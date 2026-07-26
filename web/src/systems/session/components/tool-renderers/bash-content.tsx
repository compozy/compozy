import { useState } from "react";
import { ChevronsUpDown } from "lucide-react";

import { CodeBlock } from "@agh/ui";

import type { UIMessage } from "../../types";

/** Format non-stderr output (stderr is rendered separately with error styling). */
function formatBashOutput(result: NonNullable<UIMessage["toolResult"]>): string {
  const parts: string[] = [];
  if (result.stdout) parts.push(result.stdout);
  if (result.content && !result.stdout) parts.push(result.content);
  if (result.error) parts.push(result.error);
  return parts.join("\n");
}

export function BashContent({ message }: { message: UIMessage }) {
  const command = message.toolInput?.command;
  const result = message.toolResult;
  const [expanded, setExpanded] = useState(false);

  const formattedResult = result ? formatBashOutput(result) : "";
  const totalLines = formattedResult ? formattedResult.split("\n").length : 0;
  const truncateLines = expanded ? undefined : 20;

  return (
    <div className="space-y-1.5 text-small-body" data-testid="bash-content">
      {!!command && (
        <CodeBlock code={String(command)} copyable={false} density="compact" truncateLines={4} />
      )}
      {result ? (
        <div>
          {result.stderr ? (
            <CodeBlock
              code={result.stderr}
              copyable={false}
              density="compact"
              showPrompt={false}
              tone="danger"
            />
          ) : null}
          {formattedResult ? (
            <CodeBlock
              code={formattedResult}
              copyable={false}
              density="compact"
              showPrompt={false}
              truncateLines={truncateLines}
            />
          ) : null}
          {!expanded && totalLines > 20 ? (
            <button
              type="button"
              onClick={() => setExpanded(true)}
              className="mt-1 flex items-center gap-1 text-form-label font-medium text-muted hover:text-fg transition-colors"
            >
              <ChevronsUpDown className="size-3" />
              Show full output ({totalLines} lines)
            </button>
          ) : null}
          {expanded && totalLines > 20 ? (
            <button
              type="button"
              onClick={() => setExpanded(false)}
              className="mt-1 flex items-center gap-1 text-form-label font-medium text-muted hover:text-fg transition-colors"
            >
              <ChevronsUpDown className="size-3" />
              Collapse
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
