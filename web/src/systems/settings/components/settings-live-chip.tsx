import { Pill } from "@agh/ui";

/**
 * Marks a control whose change takes effect immediately (no daemon restart),
 * pairing with the page-level restart notice for everything else.
 */
export function SettingsLiveChip() {
  return (
    <Pill data-testid="settings-live-chip" size="xs" tone="success">
      Applies now
    </Pill>
  );
}
