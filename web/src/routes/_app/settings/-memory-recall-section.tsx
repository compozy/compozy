import {
  SettingsDecimalInput,
  SettingsFieldRow,
  SettingsGroup,
  SettingsNumberInput,
  SettingValue,
} from "@/systems/settings";
import { Switch } from "@compozy/ui";
import { type ValidatedSectionProps, TEST_PREFIX } from "./-memory-settings-types";

export function RecallSection(props: ValidatedSectionProps) {
  return renderRecallSection(props);
}

function renderRecallSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Recall" help="How memories are found and ranked for each session">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-top-k`}
        label="Memories per recall"
        help="How many memories are brought into a session at once"
        error={validationErrors.recallTopK ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-top-k-input`}
            value={draft.recall.top_k}
            onValidityChange={setValidationError("recallTopK")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: { ...current.recall, top_k: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-raw-candidates`}
        label="Candidates considered"
        help="How many possible matches are checked before ranking"
        error={validationErrors.recallRawCandidates ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-raw-candidates-input`}
            value={draft.recall.raw_candidates}
            onValidityChange={setValidationError("recallRawCandidates")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: { ...current.recall, raw_candidates: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-fusion`}
        label="Ranking method"
        description="Fixed for now"
        control={<SettingValue mono>{draft.recall.fusion || "weighted"}</SettingValue>}
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-include-already-surfaced`}
        label="Include already surfaced"
        help="Re-include entries already injected this session"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-recall-include-already-surfaced-switch`}
            checked={draft.recall.include_already_surfaced}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: { ...current.recall, include_already_surfaced: checked },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-include-system`}
        label="Include _system entries"
        help="Also include files CompozyOS writes for itself (normally hidden)"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-recall-include-system-switch`}
            checked={draft.recall.include_system}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: { ...current.recall, include_system: checked },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-weight-bm25-unicode`}
        label="Weight · BM25 unicode"
        help="Score blend coefficient for the unicode FTS lane"
        error={validationErrors.recallWeightUnicode ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-weight-bm25-unicode-input`}
            value={draft.recall.weights.bm25_unicode}
            onValidityChange={setValidationError("recallWeightUnicode")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    weights: { ...current.recall.weights, bm25_unicode: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-weight-bm25-trigram`}
        label="Weight · BM25 trigram"
        help="Score blend coefficient for the trigram FTS lane"
        error={validationErrors.recallWeightTrigram ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-weight-bm25-trigram-input`}
            value={draft.recall.weights.bm25_trigram}
            onValidityChange={setValidationError("recallWeightTrigram")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    weights: { ...current.recall.weights, bm25_trigram: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-weight-recency`}
        label="Weight · recency"
        help="Score blend coefficient for the recency signal"
        error={validationErrors.recallWeightRecency ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-weight-recency-input`}
            value={draft.recall.weights.recency}
            onValidityChange={setValidationError("recallWeightRecency")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    weights: { ...current.recall.weights, recency: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-weight-recall-signal`}
        label="Weight · recall signal"
        help="Score blend coefficient for prior-recall reinforcement"
        error={validationErrors.recallWeightRecallSignal ?? undefined}
        control={
          <SettingsDecimalInput
            min={0}
            max={1}
            precision={2}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-weight-recall-signal-input`}
            value={draft.recall.weights.recall_signal}
            onValidityChange={setValidationError("recallWeightRecallSignal")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    weights: { ...current.recall.weights, recall_signal: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-banner-after-days`}
        label="Freshness banner after"
        help="Days before a memory is marked as possibly out of date"
        error={validationErrors.recallBannerAfter ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-banner-after-days-input`}
            value={draft.recall.freshness.banner_after_days}
            onValidityChange={setValidationError("recallBannerAfter")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    freshness: { ...current.recall.freshness, banner_after_days: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-signals-queue`}
        label="Signal queue capacity"
        help="How many recall events wait to be processed; the oldest are dropped when full"
        error={validationErrors.recallSignalQueue ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-32"
            data-testid={`${TEST_PREFIX}-recall-signals-queue-input`}
            value={draft.recall.signals.queue_capacity}
            onValidityChange={setValidationError("recallSignalQueue")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    signals: { ...current.recall.signals, queue_capacity: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-signals-retry`}
        label="Signal retry max"
        help="Per-update attempts before emitting a failed-signal event"
        error={validationErrors.recallSignalRetry ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-recall-signals-retry-input`}
            value={draft.recall.signals.worker_retry_max}
            onValidityChange={setValidationError("recallSignalRetry")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    signals: { ...current.recall.signals, worker_retry_max: value },
                  },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
