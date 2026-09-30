import { Play } from "lucide-react";

import {
  SettingsDecimalInput,
  SettingsFieldRow,
  SettingsGroup,
  SettingsNumberInput,
} from "@/systems/settings";
import { Button, Input, Spinner } from "@compozy/ui";
import { type ValidatedSectionProps, TEST_PREFIX } from "./-memory-settings-types";

interface TidyUpRowProps {
  dreamAvailable: boolean;
  dreamPending: boolean;
  onTriggerDream: () => void;
  actionMessage: string | null;
}

/** Primary-layer action: run the memory tidy-up (dream consolidation) now. */
export function TidyUpRow({
  dreamAvailable,
  dreamPending,
  onTriggerDream,
  actionMessage,
}: TidyUpRowProps) {
  return (
    <SettingsFieldRow
      data-testid={`${TEST_PREFIX}-dream-action`}
      label="Tidy up memory"
      description={
        actionMessage ? (
          <span data-testid={`${TEST_PREFIX}-action-message`}>{actionMessage}</span>
        ) : (
          "Reviews recent memories while you're idle and keeps the useful ones"
        )
      }
      control={
        <Button
          type="button"
          variant="secondary"
          size="sm"
          data-testid={`${TEST_PREFIX}-dream-trigger`}
          disabled={!dreamAvailable || dreamPending}
          onClick={onTriggerDream}
        >
          {dreamPending ? <Spinner className="size-3" /> : <Play className="size-3" />}
          Tidy up now
        </Button>
      }
    />
  );
}

/** Advanced tuning for the automatic tidy-up (dream) schedule and scoring. */
export function DreamSection(props: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Automatic tidy-up" help="When memory tidies itself up and what it keeps">
      <DreamScheduleFields {...props} />
      <DreamScoringFields {...props} />
    </SettingsGroup>
  );
}

function DreamScheduleFields({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <>
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-min-hours`}
        label="Min idle hours"
        help="Wait at least this many hours since the last tidy-up"
        error={validationErrors.dreamMinHours ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            precision={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-min-hours-input`}
            value={draft.dream.min_hours}
            onValidityChange={setValidationError("dreamMinHours")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: { ...current.dream, min_hours: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-min-sessions`}
        label="Min sessions"
        help="Sessions required since the last tidy-up"
        error={validationErrors.dreamMinSessions ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-min-sessions-input`}
            value={draft.dream.min_sessions}
            onValidityChange={setValidationError("dreamMinSessions")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: { ...current.dream, min_sessions: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-debounce`}
        label="Debounce"
        help="Pause after a check that found nothing to do. For example 10m"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-dream-debounce-input`}
            value={draft.dream.debounce}
            placeholder="10m"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, dream: { ...current.dream, debounce: event.target.value } };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-check-interval`}
        label="Check interval"
        help="How often memory tidies itself up while you're away. For example 30m"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-dream-check-interval-input`}
            value={draft.dream.check_interval}
            placeholder="30m"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: { ...current.dream, check_interval: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-prompt-version`}
        label="Prompt version"
        help="Version of the tidy-up instructions. Changing it lets earlier memories be reviewed again"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-dream-prompt-version-input`}
            value={draft.dream.prompt_version}
            placeholder="v1"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: { ...current.dream, prompt_version: event.target.value },
                };
              })
            }
          />
        }
      />
    </>
  );
}

function DreamScoringFields({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <>
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-gate-min-unpromoted`}
        label="Gate · min unpromoted"
        help="Recall-signal candidates that must be unpromoted to start a run"
        error={validationErrors.dreamGateMinUnpromoted ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-gate-min-unpromoted-input`}
            value={draft.dream.gates.min_unpromoted}
            onValidityChange={setValidationError("dreamGateMinUnpromoted")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    gates: { ...current.dream.gates, min_unpromoted: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-gate-min-recall-count`}
        label="Gate · min recall count"
        help="Recall events per candidate required to qualify"
        error={validationErrors.dreamGateMinRecallCount ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-gate-min-recall-count-input`}
            value={draft.dream.gates.min_recall_count}
            onValidityChange={setValidationError("dreamGateMinRecallCount")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    gates: { ...current.dream.gates, min_recall_count: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-gate-min-score`}
        label="Gate · min score"
        help="Promotion score threshold (0-1)"
        error={validationErrors.dreamGateMinScore ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-gate-min-score-input`}
            value={draft.dream.gates.min_score}
            onValidityChange={setValidationError("dreamGateMinScore")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: { ...current.dream, gates: { ...current.dream.gates, min_score: value } },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-scoring-recency-half-life`}
        label="Scoring · recency half-life (days)"
        help="Half-life applied to the recency component"
        error={validationErrors.dreamScoringHalfLife ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-scoring-recency-half-life-input`}
            value={draft.dream.scoring.recency_half_life_days}
            onValidityChange={setValidationError("dreamScoringHalfLife")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    scoring: { ...current.dream.scoring, recency_half_life_days: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-scoring-weight-frequency`}
        label="Score weight · frequency"
        error={validationErrors.dreamScoreWeightFrequency ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-scoring-weight-frequency-input`}
            value={draft.dream.scoring.weights.frequency}
            onValidityChange={setValidationError("dreamScoreWeightFrequency")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    scoring: {
                      ...current.dream.scoring,
                      weights: { ...current.dream.scoring.weights, frequency: value },
                    },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-scoring-weight-relevance`}
        label="Score weight · relevance"
        error={validationErrors.dreamScoreWeightRelevance ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-scoring-weight-relevance-input`}
            value={draft.dream.scoring.weights.relevance}
            onValidityChange={setValidationError("dreamScoreWeightRelevance")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    scoring: {
                      ...current.dream.scoring,
                      weights: { ...current.dream.scoring.weights, relevance: value },
                    },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-scoring-weight-recency`}
        label="Score weight · recency"
        error={validationErrors.dreamScoreWeightRecency ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-scoring-weight-recency-input`}
            value={draft.dream.scoring.weights.recency}
            onValidityChange={setValidationError("dreamScoreWeightRecency")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    scoring: {
                      ...current.dream.scoring,
                      weights: { ...current.dream.scoring.weights, recency: value },
                    },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-dream-scoring-weight-freshness`}
        label="Score weight · freshness"
        error={validationErrors.dreamScoreWeightFreshness ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-dream-scoring-weight-freshness-input`}
            value={draft.dream.scoring.weights.freshness}
            onValidityChange={setValidationError("dreamScoreWeightFreshness")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  dream: {
                    ...current.dream,
                    scoring: {
                      ...current.dream.scoring,
                      weights: { ...current.dream.scoring.weights, freshness: value },
                    },
                  },
                };
              })
            }
          />
        }
      />
    </>
  );
}
