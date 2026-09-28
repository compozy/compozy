import { Fragment } from "react";
import { ChevronRight, ScrollText } from "lucide-react";

import {
  Button,
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Eyebrow,
  Spinner,
} from "@compozy/ui";

import { humanizeLoopNodeId } from "../../../lib/loop-node-labels";
import {
  loopRequestContextBlock,
  type LoopRequestContextBlock,
} from "../../../lib/loop-request-context";
import type { LoopRequestView } from "../../../lib/loop-request-model";

export interface LoopRequestDetailsProps {
  view: LoopRequestView;
  fullContext?: unknown;
  error?: string;
  isLoading?: boolean;
  onRequestFull?: () => void;
  className?: string;
}

/**
 * Inspection surface for one request, closed by default: the redacted context
 * preview, the full-context fetch, and the node/generation identity live here
 * so the question itself stays plain-language.
 */
export function LoopRequestDetails({
  view,
  fullContext,
  error,
  isLoading,
  onRequestFull,
  className,
}: LoopRequestDetailsProps) {
  const preview = loopRequestContextBlock(view.request.context);
  const full = loopRequestContextBlock(fullContext);
  const canFetchFull = onRequestFull !== undefined && full === null;
  return (
    <Collapsible className={cn("group/details", className)}>
      <CollapsibleTrigger
        className="flex items-center gap-1 text-form-hint text-subtle transition-colors duration-fast hover:text-fg focus-visible:outline-none focus-visible:shadow-focus-ring"
        data-testid="loop-request-details"
      >
        <ChevronRight
          aria-hidden="true"
          className="size-3 shrink-0 transition-transform group-data-open/details:rotate-90 motion-reduce:transition-none"
        />
        Details
      </CollapsibleTrigger>
      <CollapsibleContent>
        <div
          className="mt-2 rounded-md border border-line-soft bg-input-fill px-3 py-2.5"
          data-testid="loop-request-context"
        >
          {preview ? (
            <>
              <Eyebrow className="text-faint">Context</Eyebrow>
              <ContextBody block={preview} />
            </>
          ) : null}
          {full ? (
            <div
              className={cn(preview && "mt-2.5 border-t border-line-soft pt-2.5")}
              data-testid="loop-request-context-full"
            >
              <Eyebrow className="text-faint">Full context</Eyebrow>
              <ContextBody block={full} />
            </div>
          ) : null}
          {canFetchFull ? (
            <FullContextFetch
              error={error}
              hasPreview={preview !== null}
              isLoading={isLoading}
              onRequestFull={onRequestFull}
            />
          ) : null}
          <div
            className={cn(
              "font-mono text-pill-group-badge text-faint",
              (preview || full || canFetchFull) && "mt-2.5 border-t border-line-soft pt-2.5"
            )}
          >
            {identityLine(view)}
          </div>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

function FullContextFetch({
  hasPreview,
  error,
  isLoading,
  onRequestFull,
}: {
  hasPreview: boolean;
  error?: string;
  isLoading?: boolean;
  onRequestFull: () => void;
}) {
  return (
    <div className={cn(hasPreview && "mt-2.5")}>
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
  );
}

function identityLine(view: LoopRequestView): string {
  const parts = [humanizeLoopNodeId(view.request.node_id), `round ${view.request.generation}`];
  if (view.laneLabel !== "") parts.push(view.laneLabel);
  return parts.join(" · ");
}

function ContextBody({ block }: { block: LoopRequestContextBlock }) {
  if (block.entries.length > 0) {
    return (
      <dl className="mt-1.5 grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1">
        {block.entries.map(entry => (
          <Fragment key={entry.key}>
            <dt className="font-mono text-mono-id text-faint">{entry.key}</dt>
            <dd className="min-w-0 font-mono text-mono-id break-words text-fg">{entry.value}</dd>
          </Fragment>
        ))}
      </dl>
    );
  }
  return (
    <p className="mt-1.5 max-w-[62ch] text-small-body leading-relaxed text-muted">{block.text}</p>
  );
}
