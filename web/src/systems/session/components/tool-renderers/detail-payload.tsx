import { Download, Maximize2, Scissors } from "lucide-react";
import { type ComponentProps, useState } from "react";

import { Button } from "@compozy/ui";

import { cn } from "@/lib/utils";
import {
  formatPayloadSize,
  PAYLOAD_PREVIEW_MAX_LINES,
  payloadTruncationNote,
  truncatePayload,
} from "../../lib/session-payload-truncation";
import { DetailPre } from "./detail-pre";

export interface DetailPayloadProps extends Omit<ComponentProps<"pre">, "children"> {
  /** The whole payload; the model decides how much of it renders. */
  text: string;
  /** File name for Download; the payload downloads as plain text. */
  downloadName?: string;
  maxLines?: number;
  /** Start with the whole payload shown (a find landing on text past the preview). */
  defaultExpanded?: boolean;
}

function downloadText(text: string, name: string): void {
  if (typeof document === "undefined" || typeof URL.createObjectURL !== "function") return;
  const url = URL.createObjectURL(new Blob([text], { type: "text/plain;charset=utf-8" }));
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  URL.revokeObjectURL(url);
}

/**
 * A tool payload inside the detail rail, bounded (US-019.EC-2). Past the
 * preview the strip says how much is shown of how much — "Showing the first
 * 200 of 12,480 lines · 1.8 MB" — and offers Show all (the rest renders in the
 * same scrolling body) and Download (the payload as a file). Within the bound
 * the strip never appears.
 */
export function DetailPayload({
  text,
  downloadName = "tool-output.txt",
  maxLines = PAYLOAD_PREVIEW_MAX_LINES,
  defaultExpanded = false,
  className,
  ...props
}: DetailPayloadProps) {
  const [expanded, setExpanded] = useState(defaultExpanded);
  const model = truncatePayload(text, { maxLines, expanded });
  const overflowed = model.totalLines > maxLines;
  return (
    <div className="flex min-h-0 min-w-0 flex-col gap-1" data-testid="detail-payload">
      {/* The row body bounds the whole payload at 256px (`tool-call-row-body`, a
          flex column): the output is the one child allowed to shrink, so it scrolls
          inside whatever height is left and the strip below it — counts, Show all,
          Download — stays in view instead of being clipped past the body's edge
          (task_07 VC-08). Show all renders every line in the same box. */}
      <DetailPre {...props} className={cn("min-h-0 shrink", className)}>
        {model.text}
      </DetailPre>
      {overflowed ? (
        <div
          className="flex min-w-0 items-center gap-1.5 px-0.5 text-transcript-caption text-subtle"
          data-testid="detail-payload-truncation"
          data-expanded={expanded}
        >
          <Scissors aria-hidden="true" className="size-3 shrink-0 text-faint" strokeWidth={1.75} />
          <span className="min-w-0 truncate tabular-nums" data-testid="detail-payload-note">
            {expanded
              ? `All ${model.totalLines.toLocaleString("en-US")} lines`
              : payloadTruncationNote(model)}
            <span aria-hidden="true" className="px-1.5 text-faint">
              ·
            </span>
            {formatPayloadSize(model.bytes)}
          </span>
          <span className="flex-1" />
          <Button
            data-testid="detail-payload-show-all"
            onClick={() => setExpanded(value => !value)}
            size="xs"
            type="button"
            variant="ghost"
          >
            <Maximize2 aria-hidden="true" className="size-3" />
            {expanded ? "Show less" : "Show all"}
          </Button>
          <Button
            data-testid="detail-payload-download"
            onClick={() => downloadText(text, downloadName)}
            size="xs"
            type="button"
            variant="ghost"
          >
            <Download aria-hidden="true" className="size-3" />
            Download
          </Button>
        </div>
      ) : null}
    </div>
  );
}
