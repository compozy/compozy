import type { ReactNode } from "react";

import { SettingsFieldRow, SettingsGroup, SettingsNumberInput } from "@/systems/settings";
import { Input, Switch } from "@compozy/ui";
import {
  type DraftSectionProps,
  type ValidatedSectionProps,
  TEST_PREFIX,
} from "./-memory-settings-types";

/** The page's decision layer: remember on/off, where memory lives, plus any extra rows. */
export function MemorySystemSection({
  draft,
  setDraft,
  children,
}: DraftSectionProps & { children?: ReactNode }) {
  return (
    <SettingsGroup title="Memory">
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-enabled`}
        label="Remember across sessions"
        help="Agents keep useful notes and bring them into later sessions"
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
        label="Memory folder"
        help="Where your memory files are kept"
        control={
          <Input
            className="w-72 font-mono"
            data-testid={`${TEST_PREFIX}-global-dir-input`}
            value={draft.global_dir ?? ""}
            placeholder="~/.compozy/memory"
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
      {children}
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
      help="What happens when an external memory service stops responding"
    >
      <SettingsFieldRow
        data-testid={`${TEST_PREFIX}-provider-name`}
        label="Provider name"
        description="Leave empty to use the built-in memory"
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
        description="How long to wait for the service before using the built-in memory. For example 2s"
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
        help="Failures in a row before CompozyOS stops calling the service"
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
        help="How long to wait before trying the service again. For example 30s"
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
