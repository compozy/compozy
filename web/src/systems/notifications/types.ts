import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type NotificationPresetCollection = OperationResponse<"listNotificationPresets", 200>;
export type NotificationPresetEntry = NotificationPresetCollection["presets"][number];
export type NotificationPresetTarget = NotificationPresetEntry["targets"][number];
export type NotificationPresetFilter = NonNullable<OperationQuery<"listNotificationPresets">>;
export type CreateNotificationPresetRequest = OperationRequestBody<"createNotificationPreset">;
export type UpdateNotificationPresetRequest = OperationRequestBody<"updateNotificationPreset">;
