import type { Dispatch, SetStateAction } from "react";
import { Link } from "@tanstack/react-router";

import {
  SettingLinkRow,
  SettingsFieldRow,
  SettingsGroup,
  SettingsLiveChip,
  type SettingsHooksExtensionsSection,
} from "@/systems/settings";
import { Input, Switch } from "@agh/ui";

type PolicyConfig = SettingsHooksExtensionsSection["config"];

interface PolicySectionProps {
  draft: PolicyConfig;
  setDraft: Dispatch<SetStateAction<PolicyConfig>>;
  canMutate: boolean;
}

/** Where extensions come from / Trust / Manage — the prototype's three-group split. */
export function PolicySection({ draft, setDraft, canMutate }: PolicySectionProps) {
  return (
    <>
      <SettingsGroup
        data-testid="settings-page-extensions-source-section"
        description="The marketplace this daemon trusts for extension discovery and installs."
        title="Where extensions come from"
      >
        <SettingsFieldRow
          data-testid="settings-page-extensions-policy-registry"
          description="Identifier of the marketplace publisher"
          label="Marketplace"
          control={
            <Input
              className="w-56 font-mono"
              data-testid="settings-page-extensions-policy-registry-input"
              disabled={!canMutate}
              onChange={event =>
                setDraft(current => ({
                  ...current,
                  marketplace: { ...current.marketplace, registry: event.target.value },
                }))
              }
              value={draft.marketplace.registry ?? ""}
            />
          }
        />
        <SettingsFieldRow
          data-testid="settings-page-extensions-policy-base-url"
          description="Override the registry's default endpoint"
          label="Marketplace URL"
          control={
            <Input
              className="w-72 font-mono"
              data-testid="settings-page-extensions-policy-base-url-input"
              disabled={!canMutate}
              onChange={event =>
                setDraft(current => ({
                  ...current,
                  marketplace: { ...current.marketplace, base_url: event.target.value },
                }))
              }
              placeholder="https://"
              value={draft.marketplace.base_url ?? ""}
            />
          }
        />
      </SettingsGroup>

      <SettingsGroup
        data-testid="settings-page-extensions-trust-section"
        description="Unverified extensions run with the same access as verified ones — allow them only when you trust the author."
        title="Trust"
      >
        <SettingsFieldRow
          data-testid="settings-page-extensions-policy-allow-unverified"
          description={
            <span className="inline-flex flex-wrap items-center gap-1.5">
              Unverified extensions can be installed after an explicit warning
              <SettingsLiveChip />
            </span>
          }
          label="Allow unverified extensions"
          control={
            <Switch
              aria-label="Allow unverified extensions"
              checked={draft.marketplace.allow_unverified}
              data-testid="settings-page-extensions-policy-allow-unverified-input"
              disabled={!canMutate}
              onCheckedChange={allowUnverified =>
                setDraft(current => ({
                  ...current,
                  marketplace: {
                    ...current.marketplace,
                    allow_unverified: allowUnverified,
                  },
                }))
              }
              size="sm"
            />
          }
        />
      </SettingsGroup>

      <SettingsGroup
        data-testid="settings-page-extensions-manage-section"
        description="Install, update, and remove extensions from the marketplace view."
        title="Manage"
      >
        <SettingLinkRow
          data-testid="settings-page-extensions-link-installed"
          description="Everything installed on this daemon."
          label="Installed extensions"
          render={<Link to="/marketplace/extensions" />}
        />
      </SettingsGroup>
    </>
  );
}
