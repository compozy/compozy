import { type FormEvent, useEffect, useId, useRef, useState } from "react";
import { Info } from "lucide-react";

import { Alert, AlertDescription, Button, cn, Spinner } from "@compozy/ui";

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
} from "../../../lib/loop-request-vocabulary";
import { LoopRequestAnswerForm } from "./loop-request-answer-form";
import { LoopRequestDecisionBar } from "./loop-request-decision-bar";
import { LoopRequestDetails } from "./loop-request-details";
import { LoopReviewProposedArgs } from "./loop-review-proposed-args";
import { LoopInputCatalogBoundary } from "../../input/loop-input-catalogs";
import type { LoopEntityKind } from "../../../lib/loop-input-kinds";

export interface LoopRequestCardProps {
  /** An answerable request; settled requests render as `LoopRequestSettledRow`. */
  view: LoopRequestView;
  workspaceId?: string;
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
  workspaceId = "",
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
  const promptId = `${idPrefix}-prompt`;
  const rootRef = useRef<HTMLDivElement>(null);
  const [draft, setDraft] = useState<LoopRequestDraft>(EMPTY_DRAFT);
  const isReview = view.kind === "review";

  const decision = isReview ? draft.decision : (view.decisions[0] ?? null);
  const schema = decision ? loopRequestDecisionSchema(view.request, decision) : undefined;
  const carriesPayload = decision !== null && loopRequestDecisionCarriesPayload(decision);
  const fields = carriesPayload ? loopRequestFields(schema) : [];
  const entityKinds = new Set<LoopEntityKind>();
  for (const field of fields) {
    if (field.control.kind === "entity" || field.control.kind === "entity-list") {
      entityKinds.add(field.control.entityKind);
    }
  }

  const isRaw = carriesPayload && !isLoopRequestFieldSchema(schema);

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
    <div className="px-4 py-4" data-testid="loop-request-card" ref={rootRef}>
      <h3
        className="max-w-[52ch] text-item-title font-medium text-balance text-fg-strong"
        data-testid="loop-request-prompt"
        id={promptId}
      >
        {view.request.prompt === "" ? view.title : view.request.prompt}
      </h3>
      {view.originLabel !== "" || view.laneLabel !== "" || view.expiry ? (
        <div className="mt-1 flex flex-wrap items-center gap-x-2.5 gap-y-0.5 text-form-hint text-subtle">
          {view.originLabel !== "" ? (
            <span data-testid="loop-request-origin">{view.originLabel}</span>
          ) : null}
          {view.laneLabel !== "" ? <span>{view.laneLabel}</span> : null}
          {view.expiry ? (
            <span className={cn("tabular-nums", expiryClass(view))}>{view.expiry.label}</span>
          ) : null}
        </div>
      ) : null}
      {refusal ? (
        <Alert className="mt-3" data-testid="loop-request-refusal" role="status" variant="info">
          <Info aria-hidden="true" />
          <AlertDescription>{refusal}</AlertDescription>
        </Alert>
      ) : null}
      <form className="mt-3.5 flex flex-col gap-3.5" onSubmit={submit}>
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
          <LoopInputCatalogBoundary
            workspaceId={workspaceId}
            needs={{ entities: entityKinds, runtime: false }}
          >
            <LoopRequestAnswerForm
              disabled={isPending}
              errors={{ ...draft.errors, ...fieldErrors }}
              fields={fields}
              idPrefix={idPrefix}
              isRaw={isRaw}
              onChange={changeField}
              onRawChange={raw => setDraft(previous => ({ ...previous, raw }))}
              promptId={promptId}
              rawValue={draft.raw}
              values={draft.values}
            />
          </LoopInputCatalogBoundary>
        ) : null}
        {decision ? (
          <div className="flex flex-wrap items-center gap-2.5">
            <Button
              data-testid="loop-request-submit"
              disabled={isPending}
              size="sm"
              type="submit"
              variant={decision === "reject" ? "destructive-solid" : "primary"}
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
                  Waiting for CompozyOS to record it.
                </>
              ) : null}
            </span>
          </div>
        ) : null}
      </form>
      <LoopRequestDetails
        className="mt-3.5"
        error={fullContextError}
        fullContext={fullContext}
        isLoading={isLoadingFullContext}
        onRequestFull={onRequestFullContext}
        view={view}
      />
    </div>
  );
}

function expiryClass(view: LoopRequestView): string {
  if (view.expiry?.isPast) return "text-danger";
  return view.expiry?.isNearExpiry ? "text-warning" : "";
}

function seedText(value: unknown): string {
  if (typeof value === "string") return value;
  try {
    return JSON.stringify(value, null, 2) ?? "";
  } catch {
    return "";
  }
}
