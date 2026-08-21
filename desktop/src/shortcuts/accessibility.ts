import type { AccessibilityStatus } from "./global-shortcut-types";

export const MACOS_ACCESSIBILITY_SETTINGS_URL =
  "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility";

export function detectAccessibility(options: {
  platform: NodeJS.Platform;
  isTrusted: () => boolean;
}): AccessibilityStatus {
  if (options.platform !== "darwin" || options.isTrusted()) return { allowed: true };
  return { allowed: false, settingsURL: MACOS_ACCESSIBILITY_SETTINGS_URL };
}
