import { Settings } from "lucide-react";

import { useTopbarSlot } from "@compozy/ui";

export interface UseSettingsTopbarOptions {
  status?: React.ReactNode;
  actions?: React.ReactNode;
}

/**
 * Publishes the Settings window's root head: the identity well + "Settings",
 * with the section's status and actions in the trail. Sections are siblings in
 * the always-visible sidebar, which carries the current section — the head is
 * not a drill-in trail.
 */
export function useSettingsTopbar({ status, actions }: UseSettingsTopbarOptions = {}) {
  useTopbarSlot({
    glyph: <Settings />,
    crumb: "Settings",
    status,
    actions,
  });
}
