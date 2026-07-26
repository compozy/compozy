import {
  SettingsDecimalInput,
  SettingsFieldRow,
  SettingsGroup,
  SettingsNumberInput,
} from "@/systems/settings";
import { Input, Switch } from "@agh/ui";
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
    <SettingsGroup
      title="Recall pipeline"
      description="deterministic FTS5 + scope-shadow + freshness banner"
    >
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-top-k`}
        label="Top-K"
        description="Curated entries surfaced per recall after fusion"
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
        label="Raw candidates"
        description="Pre-fusion candidate pool size pulled from each FTS lane"
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
        label="Fusion strategy"
        description="weighted is the only strategy in Slice 1; rrf is reserved for Slice 3"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-recall-fusion-input`}
            value={draft.recall.fusion}
            placeholder="weighted"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: { ...current.recall, fusion: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-include-already-surfaced`}
        label="Include already surfaced"
        description="Re-include entries already injected this session"
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
        description="Surface dreaming, extractor, and ad-hoc files (normally hidden)"
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
        description="Score blend coefficient for the unicode FTS lane"
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
        description="Score blend coefficient for the trigram FTS lane"
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
        description="Score blend coefficient for the recency signal"
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
        description="Score blend coefficient for prior-recall reinforcement"
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
        description="Days before a surfaced entry shows a staleness banner"
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
        description="Bounded post-recall signal queue; oldest entries drop on overflow"
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
        description="Per-update attempts before emitting a failed-signal event"
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
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-recall-signals-metrics`}
        label="Signal metrics"
        description="Emit recall-signal counters for observability dashboards"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-recall-signals-metrics-switch`}
            checked={draft.recall.signals.metrics_enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  recall: {
                    ...current.recall,
                    signals: { ...current.recall.signals, metrics_enabled: checked },
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
