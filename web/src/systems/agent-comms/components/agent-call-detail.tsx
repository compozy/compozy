/**
 * One call, whole, on one page.
 *
 * Reading order is the order the questions get asked: who was involved and how
 * it ended (header), what was asked (prompt), what shape the answer had to have
 * (contract), what actually came back (result), and — for the two failure
 * shapes — the evidence. The rail carries the when and the how much.
 *
 * Composition only: every decision about what may render lives in
 * `call-detail-view-model.ts`, so this file has no opinion about state and
 * cannot drift from the truthful-UI rules.
 */
import { Archive } from "lucide-react";

import { ActionResultBanner, Button, Eyebrow, MonoId, Panel } from "@compozy/ui";

import type { CostInput } from "@/lib/cost-provenance";

import { AgentCallAttempts } from "./agent-call-attempts";
import { AgentCallCost } from "./agent-call-cost";
import { AgentCallDetailHeader } from "./agent-call-detail-header";
import { AgentCallDetailTimeline } from "./agent-call-detail-timeline";
import { AgentCallResultView } from "./agent-call-result-view";
import { AgentUntrustedFrame } from "./agent-untrusted-frame";
import type { CallDetailView } from "../lib/call-detail-view-model";

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  return `${(bytes / 1024).toFixed(1)} KiB`;
}

export interface AgentCallDetailProps {
  view: CallDetailView;
  /** Usage for the child session — the one set of books delegation cost lives in. */
  childUsage: CostInput;
  /** A message the child sent that the operator has not seen in context yet. */
  untrustedNote?: { authorLabel: string; sourceId: string | null; text: string } | null;
  fullPayload?: unknown;
  onFetchFullPayload?: () => void;
  fullPayloadPending?: boolean;
  /** The whole ask, once fetched. The inline preview is bounded by the daemon. */
  fullPrompt?: string;
  onFetchFullPrompt?: () => void;
  fullPromptPending?: boolean;
  /** Whole late-result evidence, fetched only when the operator asks for it. */
  supersededPayload?: unknown;
  onFetchSuperseded?: () => void;
  supersededPending?: boolean;
  /**
   * What the last cancel actually did.
   *
   * Present for every answered cancel, not only the surprising ones: the daemon
   * always replies with the real terminal state, and saying it is the whole
   * point. `stale` means the call had already settled some other way before the
   * click landed.
   */
  cancelOutcome?: { state: string; stale: boolean } | null;
  onCancel?: () => void;
  onCallAgain?: () => void;
  onMessageChild?: () => void;
  onOpenCaller?: () => void;
  onOpenChildSession?: () => void;
  cancelPending?: boolean;
  "data-testid"?: string;
}

export function AgentCallDetail({
  view,
  childUsage,
  untrustedNote = null,
  fullPayload,
  onFetchFullPayload,
  fullPayloadPending,
  fullPrompt,
  onFetchFullPrompt,
  fullPromptPending = false,
  supersededPayload,
  onFetchSuperseded,
  supersededPending = false,
  cancelOutcome = null,
  onCancel,
  onCallAgain,
  onMessageChild,
  onOpenCaller,
  onOpenChildSession,
  cancelPending,
  "data-testid": testId,
}: AgentCallDetailProps) {
  const openChild = view.controls.openChildSession ? onOpenChildSession : undefined;
  return (
    <div data-testid={testId} data-call-id={view.callId} className="flex flex-col gap-4">
      <AgentCallDetailHeader
        view={view}
        data-testid="agent-call-detail-header"
        {...(onCancel ? { onCancel } : {})}
        {...(onCallAgain ? { onCallAgain } : {})}
        {...(onMessageChild ? { onMessageChild } : {})}
        {...(onOpenCaller ? { onOpenCaller } : {})}
        {...(openChild ? { onOpenChildSession: openChild } : {})}
        {...(cancelPending === undefined ? {} : { cancelPending })}
      />

      {/*
        The cancel receipt lives here, not in the header: the header's Cancel
        button unmounts the moment the re-read lands, and a banner beside it
        would vanish in the same tick that produced it.
      */}
      {cancelOutcome ? (
        <ActionResultBanner
          data-testid="agent-call-cancel-outcome"
          description={<span className="font-mono text-form">{cancelOutcome.state}</span>}
          title={
            cancelOutcome.stale
              ? "This call had already settled before that reached it."
              : "Canceled."
          }
          tone={cancelOutcome.stale ? "warning" : "success"}
        />
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
        <div className="flex min-w-0 flex-col gap-3">
          {view.prompt ? (
            <Panel
              title={<Eyebrow>Prompt</Eyebrow>}
              meta={<span className="font-mono text-form">{formatBytes(view.prompt.bytes)}</span>}
              data-testid="agent-call-prompt"
              {...(view.prompt.bounded && onFetchFullPrompt
                ? {
                    foot: (
                      <Button
                        disabled={fullPromptPending}
                        size="xs"
                        variant="outline"
                        type="button"
                        onClick={onFetchFullPrompt}
                      >
                        {fullPromptPending ? "Opening the ask…" : "Open the whole ask"}
                      </Button>
                    ),
                  }
                : {})}
            >
              <p className="whitespace-pre-wrap break-words text-small-body text-fg">
                {fullPrompt ?? view.prompt.preview ?? "The ask is stored but not previewed here."}
              </p>
            </Panel>
          ) : null}

          {view.expectDigest ? (
            <Panel
              title={<Eyebrow>Result contract</Eyebrow>}
              meta={<MonoId value={view.expectDigest} copy />}
              data-testid="agent-call-contract"
            >
              <p className="text-form text-muted">
                The answer had to match this shape before it could be admitted.
              </p>
            </Panel>
          ) : null}

          <AgentCallResultView
            data-testid="agent-call-result"
            result={view.result}
            verdict={view.verdict}
            {...(fullPayload === undefined ? {} : { fullPayload })}
            {...(onFetchFullPayload ? { onFetchFullPayload } : {})}
            {...(fullPayloadPending === undefined ? {} : { fullPayloadPending })}
            {...(openChild ? { onOpenChildSession: openChild } : {})}
          />

          {view.result.kind === "invalid" ? (
            <AgentCallAttempts
              data-testid="agent-call-attempts"
              repairAttempts={view.result.repairAttempts}
              firstIssueText={view.result.firstIssueText}
              secondIssueText={view.result.secondIssueText}
            />
          ) : null}

          {view.superseded ? (
            <Panel
              title={
                <span className="flex items-center gap-1.5">
                  <Archive className="size-3" aria-hidden="true" />
                  <Eyebrow>Superseded late result</Eyebrow>
                </span>
              }
              data-testid="agent-call-superseded"
            >
              <p className="text-small-body text-fg">
                The child returned {formatBytes(view.superseded.bytes)} after the call had already
                ended. Kept as evidence; it did not change the call&apos;s state.
              </p>
              {(supersededPayload ?? view.superseded.preview) !== undefined ? (
                <p className="mt-1.5 font-mono text-form text-muted">
                  {JSON.stringify(supersededPayload ?? view.superseded.preview)}
                </p>
              ) : null}
              {onFetchSuperseded && supersededPayload === undefined ? (
                <Button
                  className="mt-2"
                  disabled={supersededPending}
                  onClick={onFetchSuperseded}
                  size="xs"
                  type="button"
                  variant="outline"
                >
                  {supersededPending ? "Opening evidence…" : "Open full evidence"}
                </Button>
              ) : null}
            </Panel>
          ) : null}

          {untrustedNote ? (
            <AgentUntrustedFrame
              data-testid="agent-call-untrusted-note"
              authorLabel={untrustedNote.authorLabel}
              sourceId={untrustedNote.sourceId}
            >
              {untrustedNote.text}
            </AgentUntrustedFrame>
          ) : null}
        </div>

        <aside className="flex flex-col gap-4">
          <AgentCallDetailTimeline data-testid="agent-call-timeline" events={view.timeline} />
          <AgentCallCost data-testid="agent-call-cost" usage={childUsage} />
        </aside>
      </div>
    </div>
  );
}
