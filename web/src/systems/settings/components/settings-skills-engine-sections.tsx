import { Link } from "@tanstack/react-router";

import { Input, Switch } from "@compozy/ui";

import type { SettingsSkillsSection } from "../types";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsGroup } from "./settings-group";
import { SettingLinkRow } from "./setting-row";

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

export function SettingsSkillsMarketplaceSection({
  draft,
  onChange,
}: SettingsSkillsDraftSectionProps) {
  return (
    <SettingsGroup title="Marketplace">
      <SettingsFieldRow
        data-testid="settings-page-skills-marketplace-registry"
        label="Skills come from"
        help="Identifier of the marketplace publisher"
        control={
          <Input
            className="w-56"
            data-testid="settings-page-skills-marketplace-registry-input"
            value={draft.marketplace.registry ?? ""}
            onChange={event =>
              onChange({
                ...draft,
                marketplace: { ...draft.marketplace, registry: event.target.value },
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-skills-poll-interval"
        label="Check for updates every"
        help="How often the registry re-scans sources"
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

export function SettingsSkillsManageSection() {
  return (
    <SettingsGroup title="Manage">
      <SettingLinkRow
        data-testid="settings-page-skills-link-skills"
        label="Manage installed skills"
        render={<Link to="/marketplace/skills" />}
      />
    </SettingsGroup>
  );
}
