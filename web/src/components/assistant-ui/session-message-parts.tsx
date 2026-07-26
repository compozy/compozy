import { Activity } from "lucide-react";

import { cn } from "@/lib/utils";
import { MessageMarkdown } from "@/systems/session/components/message-markdown";
import { formatDataPreview } from "./session-message-parts.logic";

export function SessionMessageText({
  text,
  streaming = false,
}: {
  text: string;
  streaming?: boolean;
}) {
  return <MessageMarkdown content={text} streaming={streaming} />;
}

export function SessionDataEventCard({ name, data }: { name: string; data: unknown }) {
  const preview = formatDataPreview(data);
  const clippedPreview =
    preview && preview.length > 180 ? `${preview.slice(0, 180).trimEnd()}...` : preview;

  return (
    <div
      data-testid="session-data-part"
      className={cn(
        "my-2 flex w-full min-w-0 items-start gap-2 rounded-md border px-3 py-2",
        "border-line bg-canvas-soft text-form-input text-muted"
      )}
    >
      <Activity aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
      <div className="min-w-0">
        <div className="text-card-title text-fg">Data event</div>
        <div className="truncate text-form-label text-subtle">{name}</div>
        {clippedPreview ? (
          <pre className="mt-1 max-h-24 overflow-auto whitespace-pre-wrap break-words font-mono text-small-body text-muted">
            {clippedPreview}
          </pre>
        ) : null}
      </div>
    </div>
  );
}
