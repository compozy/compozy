import { Input } from "@compozy/ui";

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
    <SettingsGroup title="Endpoint & install policy" description="restart required to apply">
      <SettingsFieldRow
        data-testid="settings-page-skills-marketplace-base-url"
        label="Marketplace URL"
        help={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Override the registry&apos;s default endpoint
            <SettingsProvChip>skills.marketplace.base_url</SettingsProvChip>
          </span>
        }
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-page-skills-marketplace-base-url-input"
            value={draft.marketplace.base_url ?? ""}
            placeholder="https://"
            onChange={event =>
              onChange({
                ...draft,
                marketplace: { ...draft.marketplace, base_url: event.target.value },
              })
            }
          />
        }
      />
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
