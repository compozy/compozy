import type { ComputerId } from "@/lib/computer-scope";
import { useSettingsAutomation } from "@/systems/settings";

/**
 * The effective automation time zone for a computer (`automation.timezone`, else its
 * host zone, `_uiux.md` U4); undefined until settings load, which sentence grammar renders as UTC.
 */
export function useAutomationTimeZone(computerId?: ComputerId): string | undefined {
  const settings = useSettingsAutomation(computerId);
  return (
    settings.data?.config?.timezone?.trim() ||
    settings.data?.runtime?.host_timezone?.trim() ||
    undefined
  );
}
