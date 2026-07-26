import { CodeBlock } from "@agh/ui";

import type { UIMessage } from "../../types";

function formatInput(input: Record<string, unknown>): string {
  try {
    return JSON.stringify(input, null, 2);
  } catch {
    return String(input);
  }
}

function formatResult(result: NonNullable<UIMessage["toolResult"]>): string {
  if (result.error) return result.error;
  if (result.stdout) return result.stdout;
  if (result.content) return result.content;
  try {
    return JSON.stringify(result, null, 2);
  } catch {
    return String(result);
  }
}

export function GenericContent({ message }: { message: UIMessage }) {
  const result = message.toolResult;
  const hasResult = result && (result.stdout || result.content || result.error);

  return (
    <div className="space-y-1.5 text-small-body">
      {message.toolInput && (
        <CodeBlock
          code={formatInput(message.toolInput)}
          copyable={false}
          density="compact"
          showPrompt={false}
          truncateLines={8}
        />
      )}
      {hasResult && (
        <CodeBlock
          code={formatResult(result)}
          copyable={false}
          density="compact"
          showPrompt={false}
          truncateLines={12}
        />
      )}
    </div>
  );
}
