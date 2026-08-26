/**
 * The result pane.
 *
 * Five shapes, and none of them is a blank box. A call that finished without
 * recording anything says so in a sentence and points at the two honest ways
 * forward; a call whose answer never satisfied the contract shows the validator's
 * own words. An empty placeholder pretending a result might still appear would
 * misreport both.
 *
 * A stored-but-unpreviewed answer gets its own shape rather than borrowing the
 * resultless one. Telling an operator their agent recorded nothing, when it
 * recorded 800 KB the daemon simply declined to inline, is the worst kind of
 * wrong: it reads as a definite fact.
 *
 * When the stored payload is larger than the preview, the pane says which is
 * which — "bounded preview", with the daemon's byte count and the budget beside
 * it — and offers the full fetch as an explicit act rather than loading
 * megabytes nobody asked for.
 */
import { Download, Maximize2 } from "lucide-react";

import {
  ActionResultBanner,
  Button,
  CopyIconButton,
  Eyebrow,
  JsonViewer,
  Panel,
} from "@compozy/ui";

import { AgentCallVerdictChip } from "./agent-call-state-pill";
import type { CallResultView } from "../lib/call-detail-view-model";
import type { CallResultRow } from "../lib/call-result-rows";
import { formatAgentCallBytes } from "../lib/format-bytes";
import type { CallVerdict } from "../types";

function ResultRows({ rows }: { rows: readonly CallResultRow[] }) {
  return (
    <dl className="divide-y divide-line-soft" data-testid="agent-call-result-rows">
      {rows.map(row => (
        <div key={row.path} className="flex items-baseline gap-3 py-1.5">
          <dt className="min-w-0 shrink-0 basis-1/3 truncate font-mono text-form text-muted">
            {row.path}
          </dt>
          <dd
            className="min-w-0 flex-1 truncate font-mono text-form text-fg"
            data-summary={row.summary || undefined}
            title={row.value}
          >
            {row.value}
          </dd>
        </div>
      ))}
    </dl>
  );
}

function FullPayload({ value }: { value: unknown }) {
  return (
    <div className="mt-3" data-testid="agent-call-result-full-payload">
      <span className="mb-1 flex items-center gap-2">
        <Eyebrow>Full payload</Eyebrow>
        <CopyIconButton value={JSON.stringify(value, null, 2)} copyLabel="Copy result" />
      </span>
      <JsonViewer value={value} />
    </div>
  );
}

export interface AgentCallResultViewProps {
  result: CallResultView;
  verdict: CallVerdict | null;
  /** The whole stored payload, once the operator asked for it. */
  fullPayload?: unknown;
  onFetchFullPayload?: () => void;
  fullPayloadPending?: boolean;
  fullPayloadError?: Error | null;
  /** Absent when the child session no longer exists. */
  onOpenChildSession?: () => void;
  "data-testid"?: string;
}

export function AgentCallResultView({
  result,
  verdict,
  fullPayload,
  onFetchFullPayload,
  fullPayloadPending = false,
  fullPayloadError = null,
  onOpenChildSession,
  "data-testid": testId,
}: AgentCallResultViewProps) {
  if (result.kind === "pending") {
    return (
      <Panel data-testid={testId} title={<Eyebrow>Result</Eyebrow>}>
        <p className="text-small-body text-muted">
          Nothing has come back yet — this call is still working.
        </p>
      </Panel>
    );
  }

  if (result.kind === "none") {
    return (
      <Panel
        data-testid={testId}
        title={<Eyebrow>Result</Eyebrow>}
        foot={
          <span className="flex items-center gap-2">
            <span className="font-mono text-form text-muted">
              {result.strict ? "strict · no return act invoked" : "no return act invoked"}
            </span>
            {onOpenChildSession ? (
              <Button size="xs" variant="outline" type="button" onClick={onOpenChildSession}>
                Open child session
              </Button>
            ) : null}
          </span>
        }
      >
        <p className="text-small-body text-fg">
          The child finished without recording a result. Its transcript is intact — open the child
          session to read what it produced, or call again with the contract.
        </p>
        {result.prosePreview ? (
          <p className="mt-2 text-small-body text-muted">{result.prosePreview}</p>
        ) : null}
      </Panel>
    );
  }

  if (result.kind === "stored") {
    return (
      <Panel
        data-testid={testId}
        title={<Eyebrow>Result</Eyebrow>}
        right={<AgentCallVerdictChip verdict={verdict} />}
        foot={
          <span className="flex w-full items-center gap-2">
            <span className="font-mono text-form text-muted">
              {`${formatAgentCallBytes(result.bytes)} stored · budget ${formatAgentCallBytes(result.budgetBytes)} · overflow ${result.overflow}`}
            </span>
            <span className="flex-1" />
            {onFetchFullPayload && fullPayload === undefined ? (
              <Button
                data-testid="agent-call-result-fetch"
                size="xs"
                variant="outline"
                type="button"
                disabled={fullPayloadPending}
                onClick={onFetchFullPayload}
              >
                <Download aria-hidden="true" />
                Fetch full payload
              </Button>
            ) : null}
          </span>
        }
      >
        <p className="text-small-body text-fg">
          The answer was recorded but is too large to show here. Fetch it to read the whole thing.
        </p>
        {fullPayloadError ? (
          <ActionResultBanner
            className="mt-3"
            title="Couldn't open the full result."
            description={fullPayloadError.message}
            tone="danger"
            actions={
              onFetchFullPayload ? (
                <Button size="xs" type="button" variant="outline" onClick={onFetchFullPayload}>
                  Retry
                </Button>
              ) : null
            }
          />
        ) : null}
        {fullPayload !== undefined ? <FullPayload value={fullPayload} /> : null}
      </Panel>
    );
  }

  if (result.kind === "invalid") {
    return (
      <Panel data-testid={testId} title={<Eyebrow>Result</Eyebrow>}>
        <p className="text-small-body text-fg">
          The answer never matched what was asked, even after a retry. Both tries are on record
          below.
        </p>
      </Panel>
    );
  }

  const { shape } = result;
  return (
    <Panel
      data-testid={testId}
      title={<Eyebrow>Result</Eyebrow>}
      right={<AgentCallVerdictChip verdict={verdict} />}
      foot={
        <span className="flex w-full items-center gap-2">
          <span className="font-mono text-form text-muted">
            {result.bytes === null
              ? "size unknown"
              : `${formatAgentCallBytes(result.bytes)} stored`}
            {` · budget ${formatAgentCallBytes(result.budgetBytes)} · overflow ${result.overflow}`}
          </span>
          <span className="flex-1" />
          {onFetchFullPayload && fullPayload === undefined ? (
            <Button
              data-testid="agent-call-result-fetch"
              size="xs"
              variant="outline"
              type="button"
              disabled={fullPayloadPending}
              onClick={onFetchFullPayload}
            >
              {result.bounded ? <Download aria-hidden="true" /> : <Maximize2 aria-hidden="true" />}
              {result.bounded ? "Fetch full payload" : "Open full payload"}
            </Button>
          ) : null}
        </span>
      }
    >
      {result.bounded ? (
        <p className="mb-2 text-small-body text-muted">
          The full answer is bigger than fits here — this is a bounded preview.
        </p>
      ) : null}
      {shape.kind === "scalar" ? (
        <p className="font-mono text-form text-fg">{shape.value}</p>
      ) : shape.kind === "rows" ? (
        <ResultRows rows={shape.rows} />
      ) : null}
      {fullPayload !== undefined ? <FullPayload value={fullPayload} /> : null}
      {fullPayloadError ? (
        <ActionResultBanner
          className="mt-3"
          title="Couldn't open the full result."
          description={fullPayloadError.message}
          tone="danger"
          actions={
            onFetchFullPayload ? (
              <Button size="xs" type="button" variant="outline" onClick={onFetchFullPayload}>
                Retry
              </Button>
            ) : null
          }
        />
      ) : null}
    </Panel>
  );
}
