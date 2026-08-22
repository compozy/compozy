import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  createNotificationPreset,
  deleteNotificationPreset,
  setNotificationPresetEnablement,
  updateNotificationPreset,
} from "../adapters/notifications-api";
import { notificationKeys } from "../lib/query-keys";
import type {
  CreateNotificationPresetRequest,
  NotificationPresetEntry,
  NotificationPresetEnablement,
  SetNotificationPresetEnablementRequest,
  UpdateNotificationPresetRequest,
} from "../types";

interface NotificationPresetUpdateParams {
  name: string;
  body: UpdateNotificationPresetRequest;
  profile: string;
}

interface NotificationPresetCreateParams {
  body: CreateNotificationPresetRequest;
  profile: string;
}

interface NotificationPresetEnablementParams {
  name: string;
  body: SetNotificationPresetEnablementRequest;
}

function invalidateNotificationPresets(queryClient: ReturnType<typeof useQueryClient>) {
  return queryClient.invalidateQueries({ queryKey: notificationKeys.presetsRoot() });
}

export function useCreateNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<NotificationPresetEntry, Error, NotificationPresetCreateParams>({
    mutationFn: ({ body, profile }) => createNotificationPreset(body, profile),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useUpdateNotificationPreset() {
  const queryClient = useQueryClient();

  return useMutation<NotificationPresetEntry, Error, NotificationPresetUpdateParams>({
    mutationFn: ({ name, body, profile }) => updateNotificationPreset(name, body, profile),
    onSettled: () => invalidateNotificationPresets(queryClient),
  });
}

export function useSetNotificationPresetEnablement() {
  const queryClient = useQueryClient();

  return useMutation<NotificationPresetEnablement, Error, NotificationPresetEnablementParams>({
    mutationFn: ({ name, body }) => setNotificationPresetEnablement(name, body),
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
