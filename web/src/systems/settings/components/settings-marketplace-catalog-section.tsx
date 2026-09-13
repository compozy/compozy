import { Input } from "@compozy/ui";

import type { SettingsMarketplaceCatalogConfig } from "./marketplace-catalog-draft";
import { SettingRow } from "./setting-row";
import { SettingsGroup } from "./settings-group";

export interface SettingsMarketplaceCatalogSectionProps {
  draft: SettingsMarketplaceCatalogConfig;
  onChange: (next: SettingsMarketplaceCatalogConfig) => void;
}

const TEST_ID = "settings-page-marketplace-catalog";

/**
 * The Compozy catalog feed: where it is read from, how often, and how long a read may take. These
 * are the `marketplace.catalog.*` keys, surfaced on the page that owns the catalog.
 */
export function SettingsMarketplaceCatalogSection({
  draft,
  onChange,
}: SettingsMarketplaceCatalogSectionProps) {
  return (
    <SettingsGroup data-testid={TEST_ID} title="Compozy catalog">
      <SettingRow
        data-testid={`${TEST_ID}-base-url`}
        description="Set the base URL of the feed; the daemon reads its v3 extensions listing from there."
        error={draft.base_url.trim() === "" ? "Enter the feed URL." : undefined}
        help="marketplace.catalog.base_url"
        label="Feed URL"
        control={
          <Input
            className="w-72 font-mono"
            data-testid={`${TEST_ID}-base-url-input`}
            onChange={event => onChange({ ...draft, base_url: event.target.value })}
            placeholder="https://raw.githubusercontent.com/compozy/compozy/main/catalog"
            spellCheck={false}
            value={draft.base_url}
          />
        }
      />
      <SettingRow
        data-testid={`${TEST_ID}-ttl`}
        description="How long a read stays fresh before the catalog and its marketplaces refresh."
        error={draft.ttl.trim() === "" ? "Enter a duration." : undefined}
        help="marketplace.catalog.ttl"
        label="Refresh every"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_ID}-ttl-input`}
            onChange={event => onChange({ ...draft, ttl: event.target.value })}
            placeholder="1h"
            spellCheck={false}
            value={draft.ttl}
          />
        }
      />
      <SettingRow
        data-testid={`${TEST_ID}-timeout`}
        description="A source that does not answer in time reads as could not refresh and keeps its last read."
        error={draft.timeout.trim() === "" ? "Enter a duration." : undefined}
        help="marketplace.catalog.timeout"
        label="Timeout"
        control={
          <Input
            className="w-32 font-mono"
            data-testid={`${TEST_ID}-timeout-input`}
            onChange={event => onChange({ ...draft, timeout: event.target.value })}
            placeholder="30s"
            spellCheck={false}
            value={draft.timeout}
          />
        }
      />
    </SettingsGroup>
  );
}
