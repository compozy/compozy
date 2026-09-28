import {
  SettingsFieldRow,
  SettingsGroup,
  SettingsNumberInput,
  SettingValue,
} from "@/systems/settings";
import { Input, Switch } from "@compozy/ui";
import { type ValidatedSectionProps, TEST_PREFIX } from "./-memory-settings-types";

export function DecisionsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup title="Decisions retention">
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
        help="Keep a short summary of decisions before old ones are deleted"
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
        label="Max stored text size (bytes)"
        help="Longer text is stored as a reference instead of the full content"
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
    <SettingsGroup
      title="Memory extraction"
      help="Pulls new memories out of finished conversations"
    >
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-mode`}
        label="Mode"
        description="Fixed for now"
        control={<SettingValue mono>{draft.extractor.mode || "post_message"}</SettingValue>}
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-extractor-throttle`}
        label="Throttle turns"
        help="Skip N turns between extractor invocations"
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
        help="Time limit for each extraction. For example 60s"
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
        help="Restrict the extractor sub-agent to writes under _inbox/"
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
        help="Per-session in-flight extraction slots"
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
        help="Maximum coalesced batches before drop-oldest kicks in"
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
        description="Read-only, managed by CompozyOS"
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
        description="Read-only, managed by CompozyOS"
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
