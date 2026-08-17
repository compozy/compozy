import type { SystemNotificationState } from "@/systems/os";

const SYSTEM_STATE_NOTE: Record<SystemNotificationState, string> = {
  granted: "Only while the app is in the background",
  denied: "Permission denied — allow CompozyOS in your system notification settings",
  unsupported: "Not available in this browser",
  default: "Only while the app is in the background",
};

export function attentionSystemStateNote(state: SystemNotificationState): string {
  return SYSTEM_STATE_NOTE[state];
}
