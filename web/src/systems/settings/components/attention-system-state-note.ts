import type { SystemNotificationState } from "@/systems/os";

const SYSTEM_STATE_NOTE: Record<SystemNotificationState, string> = {
  granted: "Only while the app is in the background",
  denied:
    "Notifications are blocked by your browser or app. In-app notifications remain available.",
  unsupported: "Not available in this browser",
  default: "Only while the app is in the background",
};

export function attentionSystemStateNote(state: SystemNotificationState): string {
  return SYSTEM_STATE_NOTE[state];
}
