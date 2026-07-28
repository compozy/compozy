// Types
export type {
  CreateNotificationPresetRequest,
  NotificationPresetCollection,
  NotificationPresetEntry,
  NotificationPresetFilter,
  NotificationPresetTarget,
  UpdateNotificationPresetRequest,
} from "./types";

// Adapters
export {
  NotificationsApiError,
  createNotificationPreset,
  deleteNotificationPreset,
  listNotificationPresets,
  notificationsApi,
  updateNotificationPreset,
} from "./adapters/notifications-api";

// Query infrastructure
export { notificationKeys } from "./lib/query-keys";
export { notificationPresetsOptions, shouldRetryNotificationsQuery } from "./lib/query-options";

// Hooks
export { useNotificationPresets } from "./hooks/use-notification-presets";
export {
  useCreateNotificationPreset,
  useDeleteNotificationPreset,
  useUpdateNotificationPreset,
} from "./hooks/use-notification-preset-mutations";

// Components
export { NotificationPresetsPanel } from "./components";
