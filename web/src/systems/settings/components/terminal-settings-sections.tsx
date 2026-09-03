import type { Dispatch, SetStateAction } from "react";

import { TerminalLimitsSettingsSection } from "./terminal-limits-settings-section";
import { TerminalRecordingSettingsSection } from "./terminal-recording-settings-section";
import { TerminalRetentionSettingsSection } from "./terminal-retention-settings-section";
import { TerminalShellSettingsSection } from "./terminal-shell-settings-section";
import type { PatchTerminalSetting } from "./terminal-settings-section-types";
import type { TerminalSettingsConfig } from "../lib/terminal-settings-types";

export type { TerminalSettingsConfig } from "../lib/terminal-settings-types";

/**
 * The ten `[terminal]` keys, exactly as `config.toml` declares them.
 *
 * Agent-autonomy policy is deliberately absent: the allowlist and its tiers live
 * with the permissions surfaces where all agent-approval policy lives, and
 * projecting them here would create a second policy editor.
 */
export interface TerminalSettingsSectionsProps {
  draft: Partial<TerminalSettingsConfig>;
  setDraft: Dispatch<SetStateAction<Partial<TerminalSettingsConfig> | null>>;
  /** Per-key refusals from config validation, keyed by the TOML key. */
  validationErrors: Record<string, string | null>;
  onValidationError?: (key: keyof TerminalSettingsConfig, message: string | null) => void;
}

export function TerminalSettingsSections({
  draft,
  setDraft,
  validationErrors,
  onValidationError,
}: TerminalSettingsSectionsProps) {
  const patch: PatchTerminalSetting = (key, value) => {
    setDraft(current => (current === null ? current : { ...current, [key]: value }));
  };
  const sectionProps = { draft, onValidationError, patch, validationErrors };

  return (
    <>
      <TerminalShellSettingsSection {...sectionProps} />
      <TerminalRetentionSettingsSection {...sectionProps} />
      <TerminalRecordingSettingsSection {...sectionProps} />
      <TerminalLimitsSettingsSection {...sectionProps} />
    </>
  );
}
