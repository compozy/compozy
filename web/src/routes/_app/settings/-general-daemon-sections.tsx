import type { Dispatch, SetStateAction } from "react";

import {
  SettingRow,
  SettingsGroup,
  SettingsProvChip,
  type SettingsGeneralSection,
} from "@/systems/settings";
import { Input, Switch } from "@compozy/ui";

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

export function HttpSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup
      data-testid="settings-page-general-http"
      description="Allow other devices on your local network to use this daemon."
      title="Local network access"
    >
      <SettingRow
        data-testid="settings-page-general-http-remote-access"
        description="Binds HTTP to all network interfaces. Use only on a trusted local network."
        label={
          <>
            Allow local network access <SettingsProvChip>restart required</SettingsProvChip>
          </>
        }
        control={
          <Switch
            data-testid="settings-page-general-http-remote-access-switch"
            checked={draft.http.allow_remote_access}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  http: {
                    ...current.http,
                    host: checked ? "0.0.0.0" : "localhost",
                    allow_remote_access: checked,
                  },
                };
              })
            }
          />
        }
      />
      <SettingRow
        data-testid="settings-page-general-http-allowed-ips"
        description="Optional comma-separated IP addresses or CIDR networks. Empty allows any reachable network address."
        label="Allowed IPs or networks"
        control={
          <Input
            className="w-64 font-mono"
            data-testid="settings-page-general-http-allowed-ips-input"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                const allowedIPs = event.target.value
                  .split(",")
                  .map(value => value.trim())
                  .filter(Boolean);
                return { ...current, http: { ...current.http, allowed_ips: allowedIPs } };
              })
            }
            placeholder="192.168.1.0/24"
            value={draft.http.allowed_ips.join(", ")}
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
