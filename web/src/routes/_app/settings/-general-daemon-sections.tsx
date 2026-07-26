import type { Dispatch, SetStateAction } from "react";

import {
  SettingRow,
  SettingsGroup,
  SettingsProvChip,
  type SettingsGeneralSection,
} from "@/systems/settings";
import { Input, Switch } from "@agh/ui";

type GeneralConfig = SettingsGeneralSection["config"];

interface DraftSectionProps {
  draft: GeneralConfig;
  setDraft: Dispatch<SetStateAction<GeneralConfig | null>>;
}

export function DaemonSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup
      description="Controls periodic daemon process-memory snapshots in logs and runtime diagnostics."
      title="Runtime memory reporting"
    >
      <SettingRow
        data-testid="settings-page-general-memory-report-interval"
        description="Cadence for daemon process-memory snapshots in logs and the runtime.memory probe. Set 0s to disable memory reporting."
        label={
          <>
            Report interval <SettingsProvChip>restart required</SettingsProvChip>
          </>
        }
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
    </SettingsGroup>
  );
}

export function RedactionSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup
      data-testid="settings-page-general-redact"
      description="Controls heuristic credential redaction in agent-visible and operational text."
      title="Secret redaction"
    >
      <SettingRow
        data-testid="settings-page-general-redact-enabled"
        description="Redacts likely credentials in agent-visible text, logs, and event content. Exact secret protections remain active when disabled."
        label={
          <>
            Secret redaction heuristics <SettingsProvChip>restart required</SettingsProvChip>
          </>
        }
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
