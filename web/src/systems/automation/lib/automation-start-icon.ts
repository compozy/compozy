import { Clock3, Radio, Webhook, type LucideIcon } from "lucide-react";

import type { AutomationStart } from "./automation-sentence";

/** Kind glyph per start: `clock-3` schedule · `radio` event · `webhook` link. */
export const AUTOMATION_START_ICON: Readonly<Record<AutomationStart, LucideIcon>> = {
  schedule: Clock3,
  event: Radio,
  webhook: Webhook,
};
