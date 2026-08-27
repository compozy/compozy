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

import { ActionResultBanner, Button, Eyebrow, JsonViewer, Panel } from "@compozy/ui";

import type { CostInput } from "@/lib/cost-provenance";

import { AgentCallAttempts } from "./agent-call-attempts";
import { AgentCallContract } from "./agent-call-contract";
import { AgentCallCost } from "./agent-call-cost";
import { AgentCallDetailHeader } from "./agent-call-detail-header";
import { AgentCallDetailTimeline } from "./agent-call-detail-timeline";
import { AgentCallResultView } from "./agent-call-result-view";
import { AgentUntrustedFrame } from "./agent-untrusted-frame";
import type { CallDetailView } from "../lib/call-detail-view-model";
import { formatAgentCallBytes } from "../lib/format-bytes";

export interface AgentCallDetailProps {
  view: CallDetailView;
  /** Usage for the child session — the one set of books delegation cost lives in. */
  childUsage: CostInput;
  /** A message the child sent that the operator has not seen in context yet. */
  untrustedNote?: { authorLabel: string; text: string } | null;
  fullPayload?: unknown;
  onFetchFullPayload?: () => void;
  fullPayloadPending?: boolean;
  fullPayloadError?: Error | null;
  /** The whole ask, once fetched. The inline preview is bounded by the daemon. */
  fullPrompt?: string;
  onFetchFullPrompt?: (callId: string) => void;
  fullPromptPending?: boolean;
  fullPromptError?: Error | null;
  /** Whole late-result evidence, fetched only when the operator asks for it. */
  supersededPayload?: unknown;
  onFetchSuperseded?: () => void;
  supersededPending?: boolean;
  supersededError?: Error | null;
  /**
   * What the last cancel actually did.
   *
   * Present for every answered cancel, not only the surprising ones: the daemon
   * always replies with the real terminal state, and saying it is the whole
   * point. `stale` means the call had already settled some other way before the
   * click landed.
   */
  cancelOutcome?: { state: string; stale: boolean } | null;
  cancelFailure?: { code: string | null; message: string } | null;
  onRetryCancel?: () => void;
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
  fullPayloadError = null,
  fullPrompt,
  onFetchFullPrompt,
  fullPromptPending = false,
  fullPromptError = null,
  supersededPayload,
  onFetchSuperseded,
  supersededPending = false,
  supersededError = null,
  cancelOutcome = null,
  cancelFailure = null,
  onRetryCancel,
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

      {cancelFailure ? (
        <ActionResultBanner
          data-testid="agent-call-cancel-failure"
          title="Couldn't cancel this call."
          description={
            <span>
              {cancelFailure.message}
              {cancelFailure.code ? (
                <span className="ml-1 font-mono text-form">{cancelFailure.code}</span>
              ) : null}
            </span>
          }
          tone="danger"
          actions={
            onRetryCancel ? (
              <Button size="xs" type="button" variant="outline" onClick={onRetryCancel}>
                Retry
              </Button>
            ) : null
          }
        />
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
        <div className="flex min-w-0 flex-col gap-3">
          {view.prompt ? (
            <Panel
              className="border border-line"
              title={<Eyebrow>Prompt</Eyebrow>}
              meta={
                view.prompt.bounded ? (
                  <span className="font-mono text-form">
                    {formatAgentCallBytes(view.prompt.bytes)}
                  </span>
                ) : undefined
              }
              bodyClassName="px-3 py-3"
              data-testid="agent-call-prompt"
              {...(view.prompt.bounded && onFetchFullPrompt && fullPrompt === undefined
                ? {
                    foot: (
                      <Button
                        disabled={fullPromptPending}
                        size="xs"
                        variant="outline"
                        type="button"
                        onClick={() => onFetchFullPrompt(view.callId)}
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
              {fullPromptError ? (
                <ActionResultBanner
                  className="mt-3"
                  title="Couldn't open the whole ask."
                  description={fullPromptError.message}
                  tone="danger"
                  actions={
                    onFetchFullPrompt ? (
                      <Button
                        size="xs"
                        type="button"
                        variant="outline"
                        onClick={() => onFetchFullPrompt(view.callId)}
                      >
                        Retry
                      </Button>
                    ) : null
                  }
                />
              ) : null}
            </Panel>
          ) : null}

          {view.expectDigest ? (
            <AgentCallContract
              digest={view.expectDigest}
              budgetBytes={view.resultBudgetBytes}
              overflow={view.resultOverflow}
              data-testid="agent-call-contract"
            />
          ) : null}

          <AgentCallResultView
            data-testid="agent-call-result"
            result={view.result}
            verdict={view.verdict}
            contractDigest={view.expectDigest}
            {...(fullPayload === undefined ? {} : { fullPayload })}
            {...(onFetchFullPayload ? { onFetchFullPayload } : {})}
            {...(fullPayloadPending === undefined ? {} : { fullPayloadPending })}
            {...(fullPayloadError ? { fullPayloadError } : {})}
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
              className="border border-dashed border-line"
              title={
                <span className="flex items-center gap-1.5">
                  <Archive className="size-3 text-muted" aria-hidden="true" />
                  <Eyebrow>Superseded late result</Eyebrow>
                </span>
              }
              bodyClassName="px-3 py-3"
              data-testid="agent-call-superseded"
            >
              <p className="text-small-body text-fg">
                The child returned {formatAgentCallBytes(view.superseded.bytes)} after the call had
                already ended. Kept as evidence; it did not change the call&apos;s state.
              </p>
              {(supersededPayload ?? view.superseded.preview) !== undefined ? (
                <div className="mt-1.5">
                  <JsonViewer
                    data-testid="agent-call-superseded-payload"
                    value={supersededPayload ?? view.superseded.preview}
                  />
                </div>
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
              {supersededError ? (
                <ActionResultBanner
                  className="mt-3"
                  title="Couldn't open the late result."
                  description={supersededError.message}
                  tone="danger"
                  actions={
                    onFetchSuperseded ? (
                      <Button size="xs" type="button" variant="outline" onClick={onFetchSuperseded}>
                        Retry
                      </Button>
                    ) : null
                  }
                />
              ) : null}
            </Panel>
          ) : null}

          {untrustedNote ? (
            <AgentUntrustedFrame
              data-testid="agent-call-untrusted-note"
              authorLabel={untrustedNote.authorLabel}
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
