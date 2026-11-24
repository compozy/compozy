import type { ComputerId } from "@/lib/computer-scope";
import { useSettingsAutomation } from "@/systems/settings";

/**
 * The global `automation.timezone` every sentence surface reads (Business Rule 15);
 * undefined until settings load, which the sentence grammar renders as UTC.
 * When `computerId` is supplied, reads that target computer's effective timezone.
 */
export function useAutomationTimeZone(computerId?: ComputerId): string | undefined {
  const settings = useSettingsAutomation(computerId);
  return settings.data?.config?.timezone?.trim() || undefined;
}
