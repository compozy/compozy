import type { UIMessage } from "../../types";
import { DetailPre } from "./detail-pre";

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

/**
 * Fallback detail for uncatalogued tools: input and output as bare mono JSON
 * inside the detail rail — never a top-level card. Error output renders as
 * danger text.
 */
export function GenericContent({ message }: { message: UIMessage }) {
  const result = message.toolResult;
  const hasResult = result && (result.stdout || result.content || result.error);

  return (
    <div className="flex min-w-0 flex-col gap-1">
      {message.toolInput && Object.keys(message.toolInput).length > 0 ? (
        <DetailPre className="text-subtle">{formatInput(message.toolInput)}</DetailPre>
      ) : null}
      {hasResult ? (
        <DetailPre className={result.error ? "text-danger" : undefined}>
          {formatResult(result)}
        </DetailPre>
      ) : null}
    </div>
  );
}
