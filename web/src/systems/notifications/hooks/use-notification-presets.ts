import { useQuery } from "@tanstack/react-query";

import { notificationPresetsOptions } from "../lib/query-options";
import type { NotificationPresetFilter } from "../types";

export function useNotificationPresets(filter: NotificationPresetFilter = {}) {
  return useQuery(notificationPresetsOptions(filter));
}
