import { type FormEvent, useEffect, useId, useRef, useState } from "react";
import { Info, Lock } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertMeta,
  AlertTitle,
  Button,
  formatRelativeTime,
  Pill,
  type PillTone,
  Spinner,
} from "@compozy/ui";

import {
  loopRequestDecisionCarriesPayload,
  loopRequestDecisionSchema,
  type LoopRequestView,
} from "../../../lib/loop-request-model";
import {
  checkLoopRequestFields,
  checkLoopRequestJson,
  isLoopRequestFieldSchema,
  loopRequestFields,
  loopRequestFieldSeed,
} from "../../../lib/loop-request-payload";
import {
  LOOP_REQUEST_DECISION_LABEL,
  type LoopRequestDecision,
  type LoopRequestState,
} from "../../../lib/loop-request-vocabulary";
import { LoopRequestAnswerForm } from "./loop-request-answer-form";
import { LoopRequestContextPreview } from "./loop-request-context-preview";
import { LoopRequestDecisionBar } from "./loop-request-decision-bar";
import { LoopReviewProposedArgs } from "./loop-review-proposed-args";

export interface LoopRequestCardProps {
  view: LoopRequestView;
  isPending?: boolean;
  focusOnMount?: boolean;

  fieldErrors?: Readonly<Record<string, string>>;

  refusal?: string;

  fullContext?: unknown;
  fullContextError?: string;
  isLoadingFullContext?: boolean;
  onRequestFullContext?: () => void;
  onSubmit: (input: {
    decision: LoopRequestDecision;
    payload?: Record<string, unknown>;
    note?: string;
  }) => void;
}

const TONE_GLYPH: Record<PillTone, string> = {
  neutral: "text-muted",
  accent: "text-accent",
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
  info: "text-info",
};

const OUTCOME_TITLE: Record<LoopRequestState, string> = {
  pending: "No longer answerable",
  answered: "Answered",
  expired: "Expired",
  canceled: "Canceled",
};

interface LoopRequestDraft {
  decision: LoopRequestDecision | null;
  values: Record<string, string>;
  raw: string;
  note: string;
  errors: Record<string, string>;
}

const EMPTY_DRAFT: LoopRequestDraft = { decision: null, values: {}, raw: "", note: "", errors: {} };

export function LoopRequestCard({
  view,
  isPending,
  focusOnMount,
  fieldErrors,
  refusal,
  fullContext,
  fullContextError,
  isLoadingFullContext,
  onRequestFullContext,
  onSubmit,
}: LoopRequestCardProps) {
  const idPrefix = useId();
  const rootRef = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<LoopRequestDraft>(EMPTY_DRAFT);
  const isReview = view.kind === "review";

  const decision = isReview ? draft.decision : (view.decisions[0] ?? null);
  const schema = decision ? loopRequestDecisionSchema(view.request, decision) : undefined;
  const carriesPayload = decision !== null && loopRequestDecisionCarriesPayload(decision);
  const fields = carriesPayload ? loopRequestFields(schema) : [];

  const isRaw = carriesPayload && !isLoopRequestFieldSchema(schema);
  const Glyph = view.signal.icon;

  useEffect(() => {
    if (!focusOnMount) return;
    const root = rootRef.current;
    root?.scrollIntoView?.({ block: "center" });
    root
      ?.querySelector<HTMLElement>("form input, form select, form textarea, form button")
      ?.focus();
  }, [focusOnMount]);

  function selectDecision(next: LoopRequestDecision) {
    const nextFields = loopRequestDecisionCarriesPayload(next)
      ? loopRequestFields(loopRequestDecisionSchema(view.request, next))
      : [];

    const seed = next === "edit" ? view.request.proposed_preview : undefined;
    setDraft(previous => ({
      ...EMPTY_DRAFT,
      note: previous.note,
      decision: next,
      values: nextFields.length > 0 ? loopRequestFieldSeed(nextFields, seed) : {},
      raw: nextFields.length === 0 && seed !== undefined ? seedText(seed) : "",
    }));
  }

  function changeField(name: string, value: string) {
    setDraft(previous => {
      const errors = { ...previous.errors };
      delete errors[name];
      return { ...previous, values: { ...previous.values, [name]: value }, errors };
    });
  }

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!decision || isPending) return;
    const trimmed = draft.note.trim();
    const note = trimmed === "" ? undefined : trimmed;
    if (!carriesPayload) {
      onSubmit({ decision, note });
      return;
    }
    const check = isRaw
      ? checkLoopRequestJson(draft.raw, schema)
      : checkLoopRequestFields(fields, draft.values);
    if (!check.ok) {
      setDraft(previous => ({ ...previous, errors: check.errors }));
      return;
    }
    setDraft(previous => ({ ...previous, errors: {} }));
    onSubmit({ decision, payload: check.payload, note });
  }

  return (
    <div
      className="rounded-lg border border-line bg-canvas-soft px-4 py-3.5"
      data-testid="loop-request-card"
      ref={rootRef}
    >
      <div className="flex items-start gap-3">
        <Glyph
          aria-hidden="true"
          className={`mt-0.5 size-3.5 shrink-0 ${TONE_GLYPH[view.signal.tone]}`}
        />
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <Pill data-testid="loop-request-state" size="xs" tone={view.signal.tone}>
              <Glyph aria-hidden="true" />
              {view.signal.word}
            </Pill>
            {view.laneLabel ? (
              <Pill mono size="xs" tone="neutral">
                {view.laneLabel}
              </Pill>
            ) : null}
            {view.expiry ? (
              <span className={`font-mono text-mono-id tabular-nums ${expiryClass(view)}`}>
                {view.expiry.label}
              </span>
            ) : null}
          </div>
          <h3
            className="mt-2 text-ws-name font-medium text-fg-strong"
            data-testid="loop-request-prompt"
          >
            {view.request.prompt === "" ? view.title : view.request.prompt}
          </h3>
          <div className="mt-2.5 flex flex-col gap-3">
            <LoopRequestContextPreview
              context={view.request.context}
              fullContext={fullContext}
              error={fullContextError}
              isLoading={isLoadingFullContext}
              onRequestFull={onRequestFullContext}
            />
            {refusal ? (
              <Alert data-testid="loop-request-refusal" role="status" variant="info">
                <Info aria-hidden="true" />
                <AlertDescription>{refusal}</AlertDescription>
              </Alert>
            ) : null}
            {view.isAnswerable ? (
              <form className="flex flex-col gap-3.5" onSubmit={submit}>
                {isReview ? (
                  <LoopReviewProposedArgs
                    edited={decision === "edit" && fields.length > 0 ? draft.values : undefined}
                    proposed={view.request.proposed_preview}
                  />
                ) : null}
                {isReview ? (
                  <LoopRequestDecisionBar
                    decisions={view.decisions}
                    disabled={isPending}
                    note={draft.note}
                    onNoteChange={note => setDraft(previous => ({ ...previous, note }))}
                    onSelect={selectDecision}
                    selected={draft.decision}
                  />
                ) : null}
                {carriesPayload ? (
                  <LoopRequestAnswerForm
                    disabled={isPending}
                    errors={{ ...draft.errors, ...fieldErrors }}
                    fields={fields}
                    idPrefix={idPrefix}
                    isRaw={isRaw}
                    onChange={changeField}
                    onRawChange={raw => setDraft(previous => ({ ...previous, raw }))}
                    rawValue={draft.raw}
                    values={draft.values}
                  />
                ) : null}
                {decision ? (
                  <div className="flex flex-wrap items-center gap-2.5">
                    <Button
                      data-testid="loop-request-submit"
                      disabled={isPending}
                      size="sm"
                      type="submit"
                      variant="primary"
                    >
                      {isReview ? LOOP_REQUEST_DECISION_LABEL[decision] : "Submit answer"}
                    </Button>
                    <span
                      aria-live="polite"
                      className="inline-flex items-center gap-1.5 text-form-hint text-subtle"
                    >
                      {isPending ? (
                        <>
                          <Spinner />
                          Waiting for the daemon to record it.
                        </>
                      ) : null}
                    </span>
                  </div>
                ) : null}
              </form>
            ) : (
              <Alert data-testid="loop-request-resolution" role="group" variant={view.signal.tone}>
                <Glyph aria-hidden="true" />
                <AlertTitle>{OUTCOME_TITLE[view.state]}</AlertTitle>
                <AlertDescription>{outcomeSentence(view)}</AlertDescription>
                {view.resolution?.at ? (
                  <AlertMeta>{formatRelativeTime(view.resolution.at)}</AlertMeta>
                ) : null}
              </Alert>
            )}
          </div>
          {view.isAnswerable && !view.agentsPermitted ? (
            <p className="mt-3 flex items-center gap-1.5 text-form-hint text-subtle">
              <Lock aria-hidden="true" className="size-3 shrink-0" />
              Only operators can answer this request.
            </p>
          ) : null}
          <div className="mt-2.5 font-mono text-pill-group-badge text-faint">
            {`request · ${view.request.node_id} · gen ${view.request.generation}`}
          </div>
        </div>
      </div>
    </div>
  );
}

function expiryClass(view: LoopRequestView): string {
  if (view.expiry?.isPast) return "text-danger";
  return view.expiry?.isNearExpiry ? "text-warning" : "text-faint";
}

function outcomeSentence(view: LoopRequestView): string {
  if (view.state === "pending") return "The run ended before this request was answered.";
  if (view.state === "expired") return "The deadline passed before this request was answered.";
  const actor = [view.resolution?.actorKind, view.resolution?.actorId].filter(Boolean).join(" ");
  const answered = view.resolution?.decision ?? "";
  if (view.state === "canceled") {
    return actor === "" ? "This request was canceled." : `${actor} canceled this request.`;
  }
  if (actor !== "" && answered !== "") return `${actor} answered with ${answered}.`;
  if (actor !== "") return `${actor} answered this request.`;
  if (answered !== "") return `Answered with ${answered}.`;
  return "Someone else answered this request.";
}

function seedText(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2) ?? "";
  } catch {
    return "";
  }
}
