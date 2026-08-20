export const GLOBAL_SHORTCUT_STATUS_VALUES = [
  "registered",
  "failed_in_use",
  "failed_permission",
  "unsupported",
] as const;

export type GlobalShortcutStatus = (typeof GLOBAL_SHORTCUT_STATUS_VALUES)[number];

export interface GlobalShortcutBinding {
  command_id: string;
  chord: string;
}

export interface GlobalShortcutRegistration {
  command_id: string;
  intended_chord: string;
  status: GlobalShortcutStatus;
  active_chord?: string;
  reason?: string;
  settings_url?: string;
}

export interface AccessibilityStatus {
  allowed: boolean;
  settingsURL?: string;
}
