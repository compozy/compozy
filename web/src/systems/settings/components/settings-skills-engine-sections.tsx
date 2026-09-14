import { Input, Switch } from "@compozy/ui";

import type { SettingsSkillsSection } from "../types";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsGroup } from "./settings-group";

type SkillsConfig = SettingsSkillsSection["config"];

export interface SettingsSkillsDraftSectionProps {
  draft: SkillsConfig;
  onChange: (next: SkillsConfig) => void;
}

export function SettingsSkillsEngineSection({ draft, onChange }: SettingsSkillsDraftSectionProps) {
  return (
    <SettingsGroup title="Skills engine" description="restart required to apply">
      <SettingsFieldRow
        data-testid="settings-page-skills-enabled"
        label="Use skills"
        control={
          <Switch
            data-testid="settings-page-skills-enabled-switch"
            checked={draft.enabled}
            onCheckedChange={checked => onChange({ ...draft, enabled: checked })}
          />
        }
      />
    </SettingsGroup>
  );
}

export function SettingsSkillsDiscoverySection({
  draft,
  onChange,
}: SettingsSkillsDraftSectionProps) {
  return (
    <SettingsGroup title="Discovery">
      <SettingsFieldRow
        data-testid="settings-page-skills-poll-interval"
        label="Scan sources every"
        help="How often installed skill sources are scanned"
        control={
          <Input
            className="w-32 font-mono"
            data-testid="settings-page-skills-poll-interval-input"
            value={draft.poll_interval ?? ""}
            placeholder="5m"
            onChange={event => onChange({ ...draft, poll_interval: event.target.value })}
          />
        }
      />
    </SettingsGroup>
  );
}
