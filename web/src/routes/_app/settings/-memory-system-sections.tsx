import { SettingsFieldRow, SettingsGroup, SettingsNumberInput } from "@/systems/settings";
import { Input, Switch } from "@agh/ui";
import {
  type DraftSectionProps,
  type ValidatedSectionProps,
  TEST_PREFIX,
} from "./-memory-settings-types";

export function MemorySystemSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Memory system">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-enabled`}
        label="Memory persistence"
        description="Persist curated recall across sessions"
        control={
          <Switch
            data-testid={`${TEST_PREFIX}-enabled-switch`}
            checked={draft.enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, enabled: checked };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-global-dir`}
        label="Global memory directory"
        description="Root for global-scope memory files"
        control={
          <Input
            className="w-72 font-mono"
            data-testid={`${TEST_PREFIX}-global-dir-input`}
            value={draft.global_dir ?? ""}
            placeholder="~/.agh/memory"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  global_dir: event.target.value === "" ? undefined : event.target.value,
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

export function ProviderResilienceSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: ValidatedSectionProps) {
  return (
    <SettingsGroup
      title="Memory provider"
      description="circuit-breaker policy when an external memory provider is configured"
    >
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-provider-name`}
        label="Provider name"
        description="Empty falls back to the bundled local provider"
        control={
          <Input
            className="w-56 font-mono"
            data-testid={`${TEST_PREFIX}-provider-name-input`}
            value={draft.provider.name}
            placeholder="local"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  provider: { ...current.provider, name: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-provider-timeout`}
        label="Per-call timeout"
        description="Deadline for each provider method before failing open to local"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-provider-timeout-input`}
            value={draft.provider.timeout}
            placeholder="2s"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  provider: { ...current.provider, timeout: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-provider-failure-threshold`}
        label="Failure threshold"
        description="Consecutive failures before the breaker opens"
        error={validationErrors.providerFailureThreshold ?? undefined}
        control={
          <SettingsNumberInput
            min={1}
            className="w-24"
            data-testid={`${TEST_PREFIX}-provider-failure-threshold-input`}
            value={draft.provider.failure_threshold}
            onValidityChange={setValidationError("providerFailureThreshold")}
            onValueChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  provider: { ...current.provider, failure_threshold: value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-provider-cooldown`}
        label="Cooldown"
        description="How long the breaker stays open before retrying"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_PREFIX}-provider-cooldown-input`}
            value={draft.provider.cooldown}
            placeholder="30s"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  provider: { ...current.provider, cooldown: event.target.value },
                };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
