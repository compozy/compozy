import { NativeSelect, NativeSelectOption, Switch } from "@compozy/ui";

import {
  PROVIDER_USAGE_INTERVAL_OPTIONS,
  providerUsageIntervalValue,
  providerUsageKeychainAccess,
  providerUsageProviderEnabled,
  withProviderUsageKeychainAccess,
  withProviderUsageProviderEnabled,
  type ProviderUsageSettingsConfig,
} from "../lib/provider-usage-settings";
import type { ProviderUsageSettingsProvider } from "../hooks/use-settings-provider-usage-page";
import { ProviderLogo } from "./provider-logo";
import { SettingRow } from "./setting-row";
import { SettingsGroup } from "./settings-group";

interface DraftProps {
  draft: ProviderUsageSettingsConfig;
  onChange: (next: ProviderUsageSettingsConfig) => void;
  disabled?: boolean;
}

function intervalOptions(current: string) {
  const known = PROVIDER_USAGE_INTERVAL_OPTIONS.some(option => option.value === current);
  return known
    ? PROVIDER_USAGE_INTERVAL_OPTIONS
    : [...PROVIDER_USAGE_INTERVAL_OPTIONS, { value: current, label: current }];
}

/** Master switch and background cadence. */
export function ProviderUsageCollectionSection({ draft, onChange, disabled }: DraftProps) {
  const interval = providerUsageIntervalValue(draft.refresh_interval);
  return (
    <SettingsGroup title="Collection">
      <SettingRow
        data-testid="settings-provider-usage-enabled-row"
        label="Show provider usage"
        description="Off hides the topbar meter and stops contacting providers for usage."
        control={
          <Switch
            aria-label="Show provider usage"
            checked={draft.enabled}
            data-testid="settings-provider-usage-enabled"
            disabled={disabled}
            onCheckedChange={enabled => onChange({ ...draft, enabled })}
          />
        }
      />
      {draft.enabled ? (
        <SettingRow
          data-testid="settings-provider-usage-interval-row"
          label="Refresh in the background"
          description="Opening the usage popover or pressing Refresh still updates right away."
          control={
            <NativeSelect
              aria-label="Refresh in the background"
              data-testid="settings-provider-usage-interval"
              disabled={disabled}
              onChange={event => onChange({ ...draft, refresh_interval: event.target.value })}
              size="sm"
              value={interval}
            >
              {intervalOptions(interval).map(option => (
                <NativeSelectOption key={option.value} value={option.value}>
                  {option.label}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          }
        />
      ) : null}
    </SettingsGroup>
  );
}

/** One switch per configured provider that has a usage collector. */
export function ProviderUsageProvidersSection({
  draft,
  onChange,
  disabled,
  providers,
}: DraftProps & { providers: readonly ProviderUsageSettingsProvider[] }) {
  if (!draft.enabled || providers.length === 0) return null;
  return (
    <SettingsGroup title="Providers" data-testid="settings-provider-usage-providers">
      {providers.map(provider => (
        <SettingRow
          key={provider.id}
          data-testid={`settings-provider-usage-provider-${provider.id}-row`}
          label={
            <span className="flex items-center gap-2">
              <ProviderLogo provider={provider.id} />
              {provider.name}
            </span>
          }
          control={
            <Switch
              aria-label={`Collect ${provider.name} usage`}
              checked={providerUsageProviderEnabled(draft, provider.id)}
              data-testid={`settings-provider-usage-provider-${provider.id}`}
              disabled={disabled}
              onCheckedChange={enabled =>
                onChange(withProviderUsageProviderEnabled(draft, provider.id, enabled))
              }
            />
          }
        />
      ))}
    </SettingsGroup>
  );
}

/**
 * macOS only: consent to read Claude Code's login from the Keychain. Shown
 * when the daemon runs on macOS, or while consent is on so it can be withdrawn.
 */
export function ProviderUsageKeychainSection({
  draft,
  onChange,
  disabled,
  isMacHost,
}: DraftProps & { isMacHost: boolean }) {
  const allowed = providerUsageKeychainAccess(draft);
  if (!draft.enabled || (!isMacHost && !allowed)) return null;
  return (
    <SettingsGroup title="Privacy">
      <SettingRow
        data-testid="settings-provider-usage-keychain-row"
        label="Claude Keychain access"
        description="Reads Claude Code's login from your Keychain only when you refresh; macOS asks first."
        control={
          <Switch
            aria-label="Claude Keychain access"
            checked={allowed}
            data-testid="settings-provider-usage-keychain"
            disabled={disabled}
            onCheckedChange={checked => onChange(withProviderUsageKeychainAccess(draft, checked))}
          />
        }
      />
    </SettingsGroup>
  );
}
