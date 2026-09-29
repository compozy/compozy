import { Pill } from "@compozy/ui";

/**
 * Marks a control whose change takes effect immediately (no restart), pairing
 * with the page-level restart notice for everything else. Neutral on purpose:
 * "applies now" is information, not a recommendation.
 */
export function SettingsLiveChip() {
  return (
    <Pill data-testid="settings-live-chip" size="xs" tone="neutral">
      Applies now
    </Pill>
  );
}
