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
 * which — "bounded preview", with the daemon's byte count — and offers the full
 * fetch as an explicit act rather than loading megabytes nobody asked for.
 */
import { Download, Maximize2 } from "lucide-react";

import {
  ActionResultBanner,
  Button,
  CopyIconButton,
  Eyebrow,
  JsonViewer,
  MetadataList,
  Panel,
} from "@compozy/ui";

import { AgentCallVerdictChip } from "./agent-call-state-pill";
import type { CallResultView } from "../lib/call-detail-view-model";
import type { CallResultRow } from "../lib/call-result-rows";
import { formatAgentCallBytes } from "../lib/format-bytes";
import type { CallVerdict } from "../types";

function ResultRows({ rows }: { rows: readonly CallResultRow[] }) {
  return (
    <MetadataList data-testid="agent-call-result-rows">
      {rows.map(row => (
        <MetadataList.Row
          key={row.path}
          label={row.path}
          termProps={{ className: "font-mono" }}
          valueProps={{
            className: "truncate font-mono text-fg",
            title: row.value,
            "data-summary": row.summary || undefined,
          }}
        >
          {row.value}
        </MetadataList.Row>
      ))}
    </MetadataList>
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

function resultFoot({
  bytes,
  digest,
  fetchLabel,
  onFetchFullPayload,
  fullPayloadPending,
  fullPayload,
}: {
  bytes: string;
  digest: string | null;
  fetchLabel: string;
  onFetchFullPayload?: () => void;
  fullPayloadPending: boolean;
  fullPayload: unknown;
}) {
  return (
    <span className="flex w-full min-w-0 flex-wrap items-center gap-2">
      <span className="min-w-0 break-all font-mono text-form text-muted">
        {bytes}
        {digest ? ` · contract ${digest}` : ""}
      </span>
      {onFetchFullPayload && fullPayload === undefined ? (
        <Button
          className="ml-auto shrink-0"
          data-testid="agent-call-result-fetch"
          size="xs"
          variant="outline"
          type="button"
          disabled={fullPayloadPending}
          onClick={onFetchFullPayload}
        >
          {fetchLabel === "Fetch full payload" ? (
            <Download aria-hidden="true" />
          ) : (
            <Maximize2 aria-hidden="true" />
          )}
          {fetchLabel}
        </Button>
      ) : null}
    </span>
  );
}

export interface AgentCallResultViewProps {
  result: CallResultView;
  verdict: CallVerdict | null;
  contractDigest?: string | null;
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
  contractDigest = null,
  fullPayload,
  onFetchFullPayload,
  fullPayloadPending = false,
  fullPayloadError = null,
  onOpenChildSession,
  "data-testid": testId,
}: AgentCallResultViewProps) {
  if (result.kind === "pending") {
    return (
      <Panel
        className="border border-line"
        data-testid={testId}
        title={<Eyebrow>Result</Eyebrow>}
        bodyClassName="px-3 py-3"
      >
        <p className="text-small-body text-muted">
          Nothing has come back yet — this call is still working.
        </p>
      </Panel>
    );
  }

  if (result.kind === "none") {
    return (
      <Panel
        className="border border-line"
        data-testid={testId}
        title={<Eyebrow>Result</Eyebrow>}
        bodyClassName="px-3 py-3"
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
        className="border border-line"
        data-testid={testId}
        title={<Eyebrow>Result</Eyebrow>}
        right={<AgentCallVerdictChip verdict={verdict} />}
        bodyClassName="px-3 py-3"
        foot={resultFoot({
          bytes: `${formatAgentCallBytes(result.bytes)}`,
          digest: contractDigest,
          fetchLabel: "Fetch full payload",
          ...(onFetchFullPayload ? { onFetchFullPayload } : {}),
          fullPayloadPending,
          fullPayload,
        })}
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
      <Panel
        className="border border-line"
        data-testid={testId}
        title={<Eyebrow>Result</Eyebrow>}
        bodyClassName="px-3 py-3"
      >
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
      className="border border-line"
      data-testid={testId}
      title={<Eyebrow>Result</Eyebrow>}
      right={<AgentCallVerdictChip verdict={verdict} />}
      bodyClassName="px-3 py-3"
      foot={resultFoot({
        bytes: result.bytes === null ? "size unknown" : formatAgentCallBytes(result.bytes),
        digest: contractDigest,
        fetchLabel: result.bounded ? "Fetch full payload" : "Open full payload",
        ...(onFetchFullPayload ? { onFetchFullPayload } : {}),
        fullPayloadPending,
        fullPayload,
      })}
    >
      {result.bounded ? (
        <p className="mb-2 text-small-body text-muted">This is a bounded preview.</p>
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
