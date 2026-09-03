import type { TerminalSettingsConfig } from "../lib/terminal-settings-types";

export type PatchTerminalSetting = <K extends keyof TerminalSettingsConfig>(
  key: K,
  value: TerminalSettingsConfig[K]
) => void;

export interface TerminalSettingsSectionProps {
  draft: Partial<TerminalSettingsConfig>;
  patch: PatchTerminalSetting;
  validationErrors: Record<string, string | null>;
  onValidationError?: (key: keyof TerminalSettingsConfig, message: string | null) => void;
}
