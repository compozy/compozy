import type { NotificationPresetFilter } from "../types";

function normalizeText(value?: string | null): string {
  return value?.trim() ?? "";
}

export const notificationKeys = {
  all: ["notifications"] as const,
  presetsRoot: () => [...notificationKeys.all, "presets"] as const,
  presetsList: (filter: NotificationPresetFilter = {}) =>
    [
      ...notificationKeys.presetsRoot(),
      filter.enabled ?? "",
      filter.built_in ?? "",
      normalizeText(filter.name),
      filter.limit ?? "",
    ] as const,
};
