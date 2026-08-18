import { Fragment } from "react";
import { ScrollText } from "lucide-react";

import { Button, Eyebrow, Spinner } from "@compozy/ui";

export interface LoopRequestContextPreviewProps {
  context: unknown;
  fullContext?: unknown;
  error?: string;
  isLoading?: boolean;
  onRequestFull?: () => void;
}

interface LoopRequestContextEntry {
  key: string;
  value: string;
}

export function LoopRequestContextPreview({
  context,
  fullContext,
  error,
  isLoading,
  onRequestFull,
}: LoopRequestContextPreviewProps) {
  const preview = contextEntries(context);
  const previewText = preview.length === 0 ? contextText(context) : null;
  const full = contextEntries(fullContext);
  const fullText = full.length === 0 ? contextText(fullContext) : null;
  const hasFull = full.length > 0 || fullText !== null;
  const hasPreview = preview.length > 0 || previewText !== null;
  if (!hasPreview && !hasFull && !onRequestFull) return null;
  return (
    <div
      className="rounded-md border border-line-soft bg-input-fill px-3 py-2.5"
      data-testid="loop-request-context"
    >
      <Eyebrow className="text-faint">Context</Eyebrow>
      <ContextBody entries={preview} text={previewText} />
      {hasFull ? (
        <div
          className="mt-2.5 border-t border-line-soft pt-2.5"
          data-testid="loop-request-context-full"
        >
          <Eyebrow className="text-faint">Full context</Eyebrow>
          <ContextBody entries={full} text={fullText} />
        </div>
      ) : null}
      {onRequestFull && !hasFull ? (
        <div className="mt-2.5">
          {error ? (
            <p className="mb-2 text-small-body text-danger" role="alert">
              {error}
            </p>
          ) : null}
          <Button
            aria-busy={isLoading || undefined}
            data-testid="loop-request-context-fetch"
            disabled={isLoading}
            onClick={onRequestFull}
            size="sm"
            type="button"
            variant="outline"
          >
            {isLoading ? <Spinner /> : <ScrollText aria-hidden="true" />}
            {error ? "Try again" : "Show full context"}
          </Button>
        </div>
      ) : null}
    </div>
  );
}

function ContextBody({
  entries,
  text,
}: {
  entries: readonly LoopRequestContextEntry[];
  text: string | null;
}) {
  if (entries.length > 0) {
    return (
      <dl className="mt-1.5 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1">
        {entries.map(entry => (
          <Fragment key={entry.key}>
            <dt className="font-mono text-mono-id text-faint">{entry.key}</dt>
            <dd className="min-w-0 font-mono text-mono-id break-words text-fg">{entry.value}</dd>
          </Fragment>
        ))}
      </dl>
    );
  }
  if (text === null) return null;
  return <p className="mt-1.5 max-w-[62ch] text-small-body leading-relaxed text-muted">{text}</p>;
}

function contextEntries(value: unknown): LoopRequestContextEntry[] {
  if (typeof value !== "object" || value === null || Array.isArray(value)) return [];
  return Object.entries(value as Record<string, unknown>).map(([key, entry]) => ({
    key,
    value: printableValue(entry),
  }));
}

function contextText(value: unknown): string | null {
  if (value === undefined || value === null) return null;
  if (typeof value === "object" && !Array.isArray(value)) return null;
  const text = printableValue(value);
  return text === "" ? null : text;
}

function printableValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  try {
    return JSON.stringify(value) ?? String(value);
  } catch {
    return String(value);
  }
}
