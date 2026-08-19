import { useState } from "react";
import { ChevronLeft, ChevronRight } from "lucide-react";

import { Button, Eyebrow } from "@compozy/ui";

import { loopRequestKey, type LoopRequestView } from "../../../lib/loop-request-model";
import type { LoopRequestDecision } from "../../../lib/loop-request-vocabulary";
import { LoopRequestCard } from "./loop-request-card";
import { LoopRequestSettledRow } from "./loop-request-settled-row";

export interface LoopRequestAnswerInput {
  generation: number;
  nodeId: string;
  itemIndex: number;
  decision: LoopRequestDecision;
  payload?: Record<string, unknown>;
  note?: string;
}

export interface LoopRequestFocusTarget {
  generation?: number;
  nodeId: string;
  itemIndex: number;
}

export interface LoopRunRequestState {
  engagedKey?: string;

  isAnswerPending?: boolean;
  fieldErrors?: Readonly<Record<string, string>>;
  refusal?: string;
  fullContext?: unknown;
  fullContextError?: string;
  isLoadingFullContext?: boolean;
  onRequestFullContext?: (generation: number, nodeId: string, itemIndex: number) => void;
  onAnswer?: (input: LoopRequestAnswerInput) => void;
}

export interface LoopRequestQuestionnaireProps {
  requests: readonly LoopRequestView[];
  requestFocus?: LoopRequestFocusTarget;
  requestState?: LoopRunRequestState;
  workspaceId?: string;
}

function matchesFocus(view: LoopRequestView, focus: LoopRequestFocusTarget): boolean {
  return (
    focus.nodeId === view.request.node_id &&
    focus.itemIndex === view.request.item_index &&
    (focus.generation === undefined || focus.generation === view.request.generation)
  );
}

function engagedOpenKey(
  open: readonly LoopRequestView[],
  engagedKey: string | undefined
): string | null {
  if (!engagedKey) return null;
  return open.some(view => loopRequestKey(view.request) === engagedKey) ? engagedKey : null;
}

/**
 * One question at a time over the run's answerable requests: a progress
 * eyebrow with previous/next navigation when several are waiting, the active
 * question as a full panel, and settled requests as outcome rows below.
 * Each question still submits on its own — answers resolve from refreshed
 * daemon truth, never from an optimistic advance.
 */
export function LoopRequestQuestionnaire({
  requests,
  requestFocus,
  requestState,
  workspaceId = "",
}: LoopRequestQuestionnaireProps) {
  const open = requests.filter(view => view.isAnswerable);
  const settled = requests.filter(view => !view.isAnswerable);
  const focusView = requestFocus ? open.find(view => matchesFocus(view, requestFocus)) : undefined;
  const focusKey = focusView ? loopRequestKey(focusView.request) : null;

  const [selectedKey, setSelectedKey] = useState<string | null>(
    () => focusKey ?? engagedOpenKey(open, requestState?.engagedKey)
  );
  const [seenFocusKey, setSeenFocusKey] = useState(focusKey);
  if (focusKey !== seenFocusKey) {
    setSeenFocusKey(focusKey);
    if (focusKey !== null) setSelectedKey(focusKey);
  }

  const selectedIndex = open.findIndex(view => loopRequestKey(view.request) === selectedKey);
  const activeIndex = selectedIndex === -1 ? 0 : selectedIndex;
  const active = open[activeIndex];
  const activeKey = active === undefined ? null : loopRequestKey(active.request);
  const isEngaged = activeKey !== null && requestState?.engagedKey === activeKey;

  if (requests.length === 0) return null;

  function selectAt(index: number) {
    const target = open[index];
    if (target) setSelectedKey(loopRequestKey(target.request));
  }

  return (
    <div data-testid="loop-request-questionnaire">
      {open.length > 1 ? (
        <div className="flex items-center justify-between gap-2 border-b border-line-soft px-4 py-2">
          <Eyebrow className="text-subtle" data-testid="loop-request-progress">
            {`Question ${activeIndex + 1} of ${open.length}`}
          </Eyebrow>
          <span className="flex items-center gap-0.5">
            <Button
              aria-label="Previous question"
              data-testid="loop-request-prev"
              disabled={activeIndex === 0}
              onClick={() => selectAt(activeIndex - 1)}
              size="icon-xs"
              type="button"
              variant="ghost"
            >
              <ChevronLeft aria-hidden="true" />
            </Button>
            <Button
              aria-label="Next question"
              data-testid="loop-request-next"
              disabled={activeIndex >= open.length - 1}
              onClick={() => selectAt(activeIndex + 1)}
              size="icon-xs"
              type="button"
              variant="ghost"
            >
              <ChevronRight aria-hidden="true" />
            </Button>
          </span>
        </div>
      ) : null}
      {active !== undefined ? (
        <div
          className="motion-safe:animate-in motion-safe:fade-in-0 motion-safe:duration-base"
          key={activeKey}
        >
          <LoopRequestCard
            fieldErrors={isEngaged ? requestState?.fieldErrors : undefined}
            focusOnMount={requestFocus !== undefined && matchesFocus(active, requestFocus)}
            fullContext={isEngaged ? requestState?.fullContext : undefined}
            fullContextError={isEngaged ? requestState?.fullContextError : undefined}
            isLoadingFullContext={isEngaged ? requestState?.isLoadingFullContext : false}
            isPending={isEngaged && requestState?.isAnswerPending === true}
            onRequestFullContext={() =>
              requestState?.onRequestFullContext?.(
                active.request.generation,
                active.request.node_id,
                active.request.item_index
              )
            }
            onSubmit={input =>
              requestState?.onAnswer?.({
                ...input,
                generation: active.request.generation,
                itemIndex: active.request.item_index,
                nodeId: active.request.node_id,
              })
            }
            refusal={isEngaged ? requestState?.refusal : undefined}
            view={active}
            workspaceId={workspaceId}
          />
        </div>
      ) : null}
      {settled.map((view, index) => (
        <LoopRequestSettledRow
          key={loopRequestKey(view.request)}
          view={view}
          withDivider={active !== undefined || index > 0}
        />
      ))}
    </div>
  );
}
