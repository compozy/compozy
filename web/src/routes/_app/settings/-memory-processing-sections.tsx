import { SettingsFieldRow, SettingsGroup, SettingsNumberInput } from "@/systems/settings";
import { Input, Switch } from "@agh/ui";
import { type ValidatedSectionProps, TEST_PREFIX } from "./-memory-settings-types";

export function DecisionsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Decisions retention" description="memory_decisions WAL housekeeping">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-decisions-prune-after`}
        label="Prune after applied (days)"
        description="Delete applied decisions older than this; 0 disables pruning"
        error={validationErrors.decisionsPruneAfter ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-decisions-prune-after-input`}
            value={draft.decisions.prune_after_applied_days}
            onValidityChange={setValidationError("decisionsPruneAfter")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  decisions: { ...current.decisions, prune_after_applied_days: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-decisions-keep-summary`}
        label="Keep audit summary on prune"
        description="Emit memory.decisions.audit_summarized before deleting old rows"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-decisions-keep-summary-switch`}
            checked={draft.decisions.keep_audit_summary}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  decisions: { ...current.decisions, keep_audit_summary: checked },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-decisions-max-post-content`}
        label="Max post_content bytes"
        description="Per-row body cap; oversize rows store a content-hash reference instead"
        error={validationErrors.decisionsMaxPostBytes ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-32"
            data-testid={`${TEST_PREFIX}-decisions-max-post-content-input`}
            value={draft.decisions.max_post_content_bytes}
            onValidityChange={setValidationError("decisionsMaxPostBytes")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  decisions: { ...current.decisions, max_post_content_bytes: value },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

export function ExtractorSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Extractor" description="post-message proposal generation">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-mode`}
        label="Mode"
        description="post_message is the supported extractor mode"
        control={
          <Input
            className="w-40 font-mono"
            data-testid={`${TEST_PREFIX}-extractor-mode-input`}
            value={draft.extractor.mode}
            placeholder="post_message"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  extractor: { ...current.extractor, mode: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-throttle`}
        label="Throttle turns"
        description="Skip N turns between extractor invocations"
        error={validationErrors.extractorThrottle ?? undefined}
        control={
          <SettingsNumberInput
            min={0}
            className="w-24"
            data-testid={`${TEST_PREFIX}-extractor-throttle-input`}
            value={draft.extractor.throttle_turns}
            onValidityChange={setValidationError("extractorThrottle")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  extractor: { ...current.extractor, throttle_turns: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-deadline`}
        label="Deadline"
        description="Per-extraction wall clock budget"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-extractor-deadline-input`}
            value={draft.extractor.deadline}
            placeholder="60s"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  extractor: { ...current.extractor, deadline: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-sandbox`}
        label="Sandbox to inbox only"
        description="Restrict the extractor sub-agent to writes under _inbox/"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-extractor-sandbox-switch`}
            checked={draft.extractor.sandbox_inbox_only}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  extractor: { ...current.extractor, sandbox_inbox_only: checked },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-queue-capacity`}
        label="Queue capacity"
        description="Per-session in-flight extraction slots"
        error={validationErrors.extractorQueueCapacity ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-extractor-queue-capacity-input`}
            value={draft.extractor.queue.capacity}
            onValidityChange={setValidationError("extractorQueueCapacity")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  extractor: {
                    ...current.extractor,
                    queue: { ...current.extractor.queue, capacity: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-coalesce-max`}
        label="Coalesce ceiling"
        description="Maximum coalesced batches before drop-oldest kicks in"
        error={validationErrors.extractorCoalesce ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-extractor-coalesce-max-input`}
            value={draft.extractor.queue.coalesce_max}
            onValidityChange={setValidationError("extractorCoalesce")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  extractor: {
                    ...current.extractor,
                    queue: { ...current.extractor.queue, coalesce_max: value },
                  },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-inbox-path`}
        label="Inbox path"
        description="Read-only daemon-managed location for extractor JSONL output"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-extractor-inbox-path-input`}
            value={draft.extractor.inbox_path}
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-dlq-path`}
        label="DLQ path"
        description="Read-only daemon-managed location for extractor failure records"
        control={
          <Input
            readOnly
            className="w-full font-mono"
            data-testid={`${TEST_PREFIX}-extractor-dlq-path-input`}
            value={draft.extractor.dlq_path}
          />
        }
      />
    </SettingsGroup>
  );
}
