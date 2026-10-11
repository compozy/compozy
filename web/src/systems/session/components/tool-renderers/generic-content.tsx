import { ToolCallRow } from "@compozy/ui";

import type { UIMessage } from "../../types";
import { toolResultIsEmpty } from "../../lib/message-parts";
import { formatToolInputText, formatToolResultText } from "../../lib/tool-matched-field";
import { DetailPayload } from "./detail-payload";
import { DetailPre } from "./detail-pre";

/**
 * Fallback detail for uncatalogued tools: the input and the output each under
 * their own label, as bare mono inside the row's body card. Error output
 * renders as danger text.
 */
export function GenericContent({ message }: { message: UIMessage }) {
  const result = message.toolResult;
  const hasResult = !toolResultIsEmpty(result);
  const hasInput = Boolean(message.toolInput && Object.keys(message.toolInput).length > 0);

  return (
    <div className="flex min-h-0 min-w-0 flex-col gap-2.5">
      {hasInput && message.toolInput ? (
        <ToolCallRow.Input>
          <DetailPre className="text-subtle">{formatToolInputText(message.toolInput)}</DetailPre>
        </ToolCallRow.Input>
      ) : null}
      {hasResult && result ? (
        <ToolCallRow.Output>
          <DetailPayload
            className={result.error || result.stderr ? "text-danger" : undefined}
            downloadName={`${message.toolName ?? "tool"}-output.txt`}
            text={formatToolResultText(result)}
          />
        </ToolCallRow.Output>
      ) : null}
    </div>
  );
}
