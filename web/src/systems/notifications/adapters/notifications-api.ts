import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  CreateNotificationPresetRequest,
  NotificationPresetCollection,
  NotificationPresetEntry,
  NotificationPresetFilter,
  UpdateNotificationPresetRequest,
} from "../types";

export class NotificationsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "NotificationsApiError";
  }
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

function normalizeNotificationPresetFilter(filter: NotificationPresetFilter = {}) {
  return {
    enabled: filter.enabled,
    built_in: filter.built_in,
    name: normalizeOptionalText(filter.name),
    limit: filter.limit,
  };
}

export async function listNotificationPresets(
  filter: NotificationPresetFilter = {},
  signal?: AbortSignal
): Promise<NotificationPresetCollection> {
  const { data, error, response } = await apiClient.GET("/api/notifications/presets", {
    params: { query: normalizeNotificationPresetFilter(filter) },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new NotificationsApiError(
      defaultApiErrorMessage("Failed to load notification presets", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to load notification presets");
}

export async function createNotificationPreset(
  body: CreateNotificationPresetRequest,
  signal?: AbortSignal
): Promise<NotificationPresetEntry> {
  const { data, error, response } = await apiClient.POST("/api/notifications/presets", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new NotificationsApiError(
      defaultApiErrorMessage("Failed to create notification preset", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create notification preset").preset;
}

export async function updateNotificationPreset(
  name: string,
  body: UpdateNotificationPresetRequest,
  signal?: AbortSignal
): Promise<NotificationPresetEntry> {
  const { data, error, response } = await apiClient.PUT("/api/notifications/presets/{name}", {
    params: { path: { name } },
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new NotificationsApiError(
      defaultApiErrorMessage("Failed to update notification preset", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to update notification preset").preset;
}

export async function deleteNotificationPreset(name: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/notifications/presets/{name}", {
    params: { path: { name } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new NotificationsApiError(
      defaultApiErrorMessage("Failed to delete notification preset", response, error),
      response.status
    );
  }
}

export const notificationsApi = {
  listPresets: listNotificationPresets,
  createPreset: createNotificationPreset,
  updatePreset: updateNotificationPreset,
  deletePreset: deleteNotificationPreset,
};
