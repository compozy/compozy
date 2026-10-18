import { useSettingsAutomation } from "@/systems/settings";

/**
 * The global `automation.timezone` every sentence surface reads (Business Rule 15);
 * undefined until settings load, which the sentence grammar renders as UTC.
 */
export function useAutomationTimeZone(): string | undefined {
  const settings = useSettingsAutomation();
  return settings.data?.config?.timezone?.trim() || undefined;
}
