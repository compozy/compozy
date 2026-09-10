import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import { NotificationsApiError } from "./notifications-api";
import type {
  AttentionNotifications,
  AcknowledgeAttentionRequest,
  AttentionNotificationScope,
} from "../types";

export async function listAttentionNotifications(
  profile: string,
  signal?: AbortSignal
): Promise<AttentionNotifications> {
  const { data, error, response } = await apiClient.GET("/api/notifications/attention", {
    params: { query: { profile, receipt_profile: profile } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new NotificationsApiError(
      defaultApiErrorMessage("Failed to load notifications", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to load notifications");
}

export async function acknowledgeAttentionNotifications(
  scope: AttentionNotificationScope,
  body: AcknowledgeAttentionRequest,
  signal?: AbortSignal
): Promise<void> {
  const { error, response } = await apiClient.POST("/api/notifications/attention/acknowledge", {
    params: { query: scope },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new NotificationsApiError(
      defaultApiErrorMessage("Could not clear notifications. Try again.", response, error),
      response.status
    );
  }
}
