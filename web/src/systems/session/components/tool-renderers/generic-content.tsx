import type { UIMessage } from "../../types";
import { toolResultIsEmpty } from "../../lib/message-parts";
import { formatToolInputText, formatToolResultText } from "../../lib/tool-matched-field";
import { DetailPayload } from "./detail-payload";
import { DetailPre } from "./detail-pre";

/**
 * Fallback detail for uncatalogued tools: input and output as bare mono JSON
 * inside the detail rail — never a top-level card. Error output renders as
 * danger text.
 */
export function GenericContent({ message }: { message: UIMessage }) {
  const result = message.toolResult;
  const hasResult = !toolResultIsEmpty(result);

  return (
    <div className="flex min-w-0 flex-col gap-1">
      {message.toolInput && Object.keys(message.toolInput).length > 0 ? (
        <DetailPre className="text-subtle">{formatToolInputText(message.toolInput)}</DetailPre>
      ) : null}
      {hasResult && result ? (
        <DetailPayload
          className={result.error || result.stderr ? "text-danger" : undefined}
          downloadName={`${message.toolName ?? "tool"}-output.txt`}
          text={formatToolResultText(result)}
        />
      ) : null}
    </div>
  );
}
