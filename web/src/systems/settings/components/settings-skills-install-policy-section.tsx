import type { SettingsSkillsDraftSectionProps } from "./settings-skills-engine-sections";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsGroup } from "./settings-group";
import { SettingsProvChip } from "./settings-advanced-fold";
import { SettingsTaglistField } from "./settings-taglist-field";

export function SettingsSkillsInstallPolicySection({
  draft,
  onChange,
}: SettingsSkillsDraftSectionProps) {
  return (
    <SettingsGroup title="Install policy" description="restart required to apply">
      <SettingsFieldRow
        data-testid="settings-page-skills-allowed-mcp"
        label="Allowed MCP installs"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Marketplace MCP packages that may be installed
            <SettingsProvChip>skills.allowed_marketplace_mcp</SettingsProvChip>
          </span>
        }
        control={
          <SettingsTaglistField
            data-testid="settings-page-skills-allowed-mcp-input"
            label="Allowed MCP installs"
            value={draft.allowed_marketplace_mcp ?? []}
            onChange={value => onChange({ ...draft, allowed_marketplace_mcp: value })}
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-skills-allowed-hooks"
        label="Allowed hook installs"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Marketplace hook packages that may be installed
            <SettingsProvChip>skills.allowed_marketplace_hooks</SettingsProvChip>
          </span>
        }
        control={
          <SettingsTaglistField
            data-testid="settings-page-skills-allowed-hooks-input"
            label="Allowed hook installs"
            value={draft.allowed_marketplace_hooks ?? []}
            onChange={value => onChange({ ...draft, allowed_marketplace_hooks: value })}
          />
        }
      />
    </SettingsGroup>
  );
}
