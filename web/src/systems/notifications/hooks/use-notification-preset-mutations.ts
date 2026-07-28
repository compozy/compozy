import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  createNotificationPreset,
  deleteNotificationPreset,
  updateNotificationPreset,
} from "../adapters/notifications-api";
import { notificationKeys } from "../lib/query-keys";
import type {
  CreateNotificationPresetRequest,
  NotificationPresetEntry,
  UpdateNotificationPresetRequest,
} from "../types";

interface NotificationPresetUpdateParams {
  name: string;
  body: UpdateNotificationPresetRequest;
}

function invalidateNotificationPresets(queryClient: ReturnType<typeof useQueryClient>) {
  return queryClient.invalidateQueries({ queryKey: notificationKeys.presetsRoot() });
}

export function useCreateNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<NotificationPresetEntry, Error, CreateNotificationPresetRequest>({
    mutationFn: body => createNotificationPreset(body),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useUpdateNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<NotificationPresetEntry, Error, NotificationPresetUpdateParams>({
    mutationFn: ({ name, body }) => updateNotificationPreset(name, body),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useDeleteNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: name => deleteNotificationPreset(name),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}
