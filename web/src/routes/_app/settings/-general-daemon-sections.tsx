import type { Dispatch, SetStateAction } from "react";

import { SettingRow, SettingsGroup, type SettingsGeneralSection } from "@/systems/settings";
import { Input, Switch } from "@compozy/ui";

type GeneralConfig = SettingsGeneralSection["config"];

interface DraftSectionProps {
  draft: GeneralConfig;
  setDraft: Dispatch<SetStateAction<GeneralConfig | null>>;
}

/** Memory-usage logging cadence — an operator knob, rendered as a row inside General's Advanced fold. */
export function MemoryReportRow({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingRow
      data-testid="settings-page-general-memory-report-interval"
      help="Applies after CompozyOS restarts."
      description="How often CompozyOS writes its memory use to the logs. 0 turns it off."
      label="Memory usage logging"
      control={
        <Input
          className="w-32 font-mono"
          data-testid="settings-page-general-memory-report-interval-input"
          value={draft.daemon.memory_report_interval}
          placeholder="5m"
          onChange={event =>
            setDraft(prev => {
              const current = prev ?? draft;
              return {
                ...current,
                daemon: { ...current.daemon, memory_report_interval: event.target.value },
              };
            })
          }
        />
      }
    />
  );
}

export function RedactionSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup data-testid="settings-page-general-redact" title="Privacy">
      <SettingRow
        data-testid="settings-page-general-redact-enabled"
        help="Applies after CompozyOS restarts."
        description="Masks text that looks like a password or API key. Known secrets are always hidden."
        label="Hide passwords and keys in logs"
        control={
          <Switch
            data-testid="settings-page-general-redact-enabled-switch"
            checked={draft.redact.enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, redact: { ...current.redact, enabled: checked } };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
