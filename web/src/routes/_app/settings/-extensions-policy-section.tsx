import type { Dispatch, SetStateAction } from "react";
import { Link } from "@tanstack/react-router";

import {
  SettingLinkRow,
  SettingsFieldRow,
  SettingsGroup,
  SettingsLiveChip,
  type SettingsHooksExtensionsSection,
} from "@/systems/settings";
import { Input, Switch } from "@compozy/ui";

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
        title="Where extensions come from"
      >
        <SettingsFieldRow
          data-testid="settings-page-extensions-policy-github-enabled"
          help="Install extensions from GitHub releases"
          label="GitHub"
          control={
            <Switch
              aria-label="GitHub"
              checked={draft.sources.github.enabled}
              data-testid="settings-page-extensions-policy-github-enabled-input"
              disabled={!canMutate}
              onCheckedChange={enabled =>
                setDraft(current => ({
                  ...current,
                  sources: {
                    ...current.sources,
                    github: { ...current.sources.github, enabled },
                  },
                }))
              }
              size="sm"
            />
          }
        />
        <SettingsFieldRow
          data-testid="settings-page-extensions-policy-github-base-url"
          help="Endpoint used for GitHub installs and search"
          label="GitHub API URL"
          control={
            <Input
              className="w-72 font-mono"
              data-testid="settings-page-extensions-policy-github-base-url-input"
              disabled={!canMutate}
              onChange={event =>
                setDraft(current => ({
                  ...current,
                  sources: {
                    ...current.sources,
                    github: { ...current.sources.github, base_url: event.target.value },
                  },
                }))
              }
              placeholder="https://api.github.com"
              value={draft.sources.github.base_url}
            />
          }
        />
        <SettingsFieldRow
          data-testid="settings-page-extensions-policy-git-enabled"
          help="Install extensions by cloning a Git repository"
          label="Git"
          control={
            <Switch
              aria-label="Git"
              checked={draft.sources.git.enabled}
              data-testid="settings-page-extensions-policy-git-enabled-input"
              disabled={!canMutate}
              onCheckedChange={enabled =>
                setDraft(current => ({
                  ...current,
                  sources: { ...current.sources, git: { ...current.sources.git, enabled } },
                }))
              }
              size="sm"
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
          help="Unverified extensions can be installed after an explicit warning"
          description={<SettingsLiveChip />}
          label="Allow unverified extensions"
          control={
            <Switch
              aria-label="Allow unverified extensions"
              checked={draft.trust.allow_unverified}
              data-testid="settings-page-extensions-policy-allow-unverified-input"
              disabled={!canMutate}
              onCheckedChange={allowUnverified =>
                setDraft(current => ({
                  ...current,
                  trust: { ...current.trust, allow_unverified: allowUnverified },
                }))
              }
              size="sm"
            />
          }
        />
      </SettingsGroup>

      <SettingsGroup data-testid="settings-page-extensions-manage-section" title="Manage">
        <SettingLinkRow
          data-testid="settings-page-extensions-link-installed"
          label="Installed extensions"
          render={<Link to="/marketplace/extensions" />}
        />
      </SettingsGroup>
    </>
  );
}
