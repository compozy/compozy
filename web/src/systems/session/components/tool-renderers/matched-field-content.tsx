import { Eyebrow } from "@compozy/ui";

import { matchedToolFieldText, type SessionToolBodyField } from "../../lib/tool-matched-field";
import type { UIMessage } from "../../types";
import { DetailPayload } from "./detail-payload";

const FIELD_LABELS: Record<SessionToolBodyField, string> = {
  error: "Error",
  input: "Input",
  output: "Output",
};

/**
 * The field a find jump matched (S8), shown in full beside the tool's own
 * display: the daemon searched the raw input / output / error, so the reader
 * sees exactly that payload — untruncated, so a hit past the bounded preview
 * is on screen — through the same detail composition every tool uses.
 */
export function MatchedToolFieldContent({
  message,
  field,
}: {
  message: UIMessage;
  field: SessionToolBodyField;
}) {
  const text = matchedToolFieldText(message, field);
  if (text === null) return null;
  return (
    <div
      className="flex min-w-0 flex-col gap-1.5"
      data-field={field}
      data-testid="tool-matched-field"
    >
      <Eyebrow className="text-subtle">{FIELD_LABELS[field]}</Eyebrow>
      <DetailPayload
        className={field === "error" ? "text-danger" : "text-subtle"}
        defaultExpanded
        downloadName={`${message.toolName ?? "tool"}-${field}.txt`}
        text={text}
      />
    </div>
  );
}
