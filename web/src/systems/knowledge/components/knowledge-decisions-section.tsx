import { AlertCircle, History, RotateCcw } from "lucide-react";

import { Button, Empty, Pill, Section, Spinner, Time, TimelineEvent } from "@agh/ui";

import { decisionOpLabel, decisionSourceLabel } from "@/systems/knowledge/lib/knowledge-formatters";
import type { MemoryDecision } from "@/systems/knowledge/types";

import { pillToneFromDecisionOp, pillToneFromDecisionSource } from "./knowledge-pill-tone";

interface KnowledgeDecisionsSectionProps {
  decisions: MemoryDecision[] | undefined;
  isLoading: boolean;
  error: Error | null;
  onRevertDecision?: (decision: MemoryDecision) => Promise<void>;
  revertingDecisionId?: string | null;
  revertError?: string | null;
}

function KnowledgeDecisionsSection({
  decisions,
  isLoading,
  error,
  onRevertDecision,
  revertingDecisionId = null,
  revertError = null,
}: KnowledgeDecisionsSectionProps) {
  const handleRevert = async (decision: MemoryDecision) => {
    if (!onRevertDecision) return;
    try {
      await onRevertDecision(decision);
    } catch {
      // Error state is surfaced through `revertError`.
    }
  };

  return (
    <Section data-testid="knowledge-decisions-section" label="Recent controller decisions">
      {isLoading ? (
        <div
          className="flex items-center gap-2 px-1 py-3 text-xs text-subtle"
          data-testid="knowledge-decisions-loading"
        >
          <Spinner /> Loading decisions…
        </div>
      ) : error ? (
        <Empty
          className="max-w-md"
          data-testid="knowledge-decisions-error"
          description={error.message ?? "Failed to load decisions"}
          icon={AlertCircle}
          title="Unable to load decisions"
        />
      ) : !decisions || decisions.length === 0 ? (
        <Empty
          className="max-w-md"
          data-testid="knowledge-decisions-empty"
          description="No controller decisions recorded for this memory yet."
          icon={History}
          title="No decisions"
        />
      ) : (
        <ul
          className="flex flex-col gap-1"
          data-testid="knowledge-decisions-list"
          aria-label="Controller decisions"
        >
          {decisions.map(decision => {
            const opTone = pillToneFromDecisionOp(decision.op);
            const sourceTone = pillToneFromDecisionSource(decision.source);
            const isReverting = revertingDecisionId === decision.id;
            const showRevert = Boolean(onRevertDecision) && Boolean(decision.applied_at);
            return (
              <TimelineEvent
                data-testid={`knowledge-decision-${decision.id}`}
                key={decision.id}
                tone={opTone}
                title={
                  <div className="flex min-w-0 flex-wrap items-center gap-1.5">
                    <Pill mono data-testid={`knowledge-decision-op-${decision.id}`} tone={opTone}>
                      {decisionOpLabel(decision.op)}
                    </Pill>
                    <Pill
                      mono
                      data-testid={`knowledge-decision-source-${decision.id}`}
                      tone={sourceTone}
                    >
                      {decisionSourceLabel(decision.source)}
                    </Pill>
                  </div>
                }
                time={
                  <Time
                    data-testid={`knowledge-decision-time-${decision.id}`}
                    iso={decision.decided_at}
                  />
                }
                description={decision.reason ?? undefined}
                meta={
                  <>
                    <span data-testid={`knowledge-decision-confidence-${decision.id}`}>
                      Confidence {decision.confidence.toFixed(2)}
                    </span>
                    {decision.applied_at ? (
                      <span data-testid={`knowledge-decision-applied-${decision.id}`}>
                        Applied <Time iso={decision.applied_at} />
                      </span>
                    ) : (
                      <span data-testid={`knowledge-decision-pending-${decision.id}`}>
                        Not applied
                      </span>
                    )}
                    {decision.target_filename ? (
                      <span data-testid={`knowledge-decision-target-${decision.id}`}>
                        Target {decision.target_filename}
                      </span>
                    ) : null}
                    {showRevert ? (
                      <Button
                        className="ml-auto"
                        data-testid={`revert-memory-decision-${decision.id}`}
                        disabled={isReverting}
                        onClick={() => void handleRevert(decision)}
                        size="sm"
                        type="button"
                        variant="ghost"
                      >
                        {isReverting ? (
                          <Spinner aria-hidden="true" className="size-3" />
                        ) : (
                          <RotateCcw className="size-3" />
                        )}
                        Revert
                      </Button>
                    ) : null}
                  </>
                }
              >
                {revertError && isReverting ? (
                  <p
                    className="text-small-body text-danger"
                    data-testid={`knowledge-decision-revert-error-${decision.id}`}
                  >
                    {revertError}
                  </p>
                ) : null}
              </TimelineEvent>
            );
          })}
        </ul>
      )}
    </Section>
  );
}

export { KnowledgeDecisionsSection };
export type { KnowledgeDecisionsSectionProps };
