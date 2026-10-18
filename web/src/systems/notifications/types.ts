import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type AttentionNotifications = OperationResponse<"listAttentionNotifications", 200>;
export type AttentionNotification = AttentionNotifications["items"][number];
export type AcknowledgeAttentionRequest = OperationRequestBody<"acknowledgeAttentionNotifications">;
export type AttentionNotificationScope = NonNullable<
  OperationQuery<"acknowledgeAttentionNotifications">
>;
