import { useCallback, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { accountKeys } from "@/systems/account";

import {
  patchRemoteDevice,
  patchRemoteStatus,
  revokeRemoteDevice,
} from "../adapters/remote-host-api";
import { extractLocalPendingApprovals, toRemoteDeviceView } from "../lib/computer-view";
import { computerRegistryKeys } from "../lib/query-keys";
import { remoteDevicesOptions, remoteStatusOptions } from "../lib/query-options";
import type {
  AccountDeviceFixture,
  PendingApprovalFixture,
  RemoteScope,
} from "../mocks/remote-computers-types";

export function useLocalRemoteControls({
  signedIn,
  currentDeviceId,
}: {
  signedIn: boolean;
  currentDeviceId: string | null;
}) {
  const queryClient = useQueryClient();

  const remoteStatusQuery = useQuery(remoteStatusOptions(signedIn));
  const remoteDevicesQuery = useQuery(
    remoteDevicesOptions(signedIn && Boolean(remoteStatusQuery.data?.enabled))
  );

  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(null);
  const [revokeApprovalFailure, setRevokeApprovalFailure] = useState<string | null>(null);

  const localDevices = useMemo<AccountDeviceFixture[]>(
    () =>
      (remoteDevicesQuery.data?.devices ?? []).map(device =>
        toRemoteDeviceView(device, currentDeviceId)
      ),
    [remoteDevicesQuery.data?.devices, currentDeviceId]
  );

  const localPendingApprovals = useMemo<PendingApprovalFixture[]>(
    () => extractLocalPendingApprovals(remoteDevicesQuery.data?.devices ?? []),
    [remoteDevicesQuery.data?.devices]
  );

  const remotePatchMutation = useMutation({
    mutationFn: (body: Parameters<typeof patchRemoteStatus>[0]) => patchRemoteStatus(body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to update remote access.");
    },
  });

  const localDevicePatchMutation = useMutation({
    mutationFn: ({
      deviceId,
      body,
    }: {
      deviceId: string;
      body: Parameters<typeof patchRemoteDevice>[1];
    }) => patchRemoteDevice(deviceId, body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to update device.");
    },
  });

  const revokeLocalDeviceMutation = useMutation({
    mutationFn: async ({
      device,
      enableApproval,
    }: {
      device: AccountDeviceFixture;
      enableApproval: boolean;
    }) => {
      await revokeRemoteDevice(device.id);
      if (enableApproval) {
        try {
          await patchRemoteStatus({ require_device_approval: true });
        } catch (err) {
          return {
            approvalFailed:
              err instanceof Error ? err.message : "Couldn't turn on device approval.",
          };
        }
      }
      return { approvalFailed: null as string | null };
    },
    onSuccess: (result, vars) => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      void queryClient.invalidateQueries({ queryKey: accountKeys.devices() });
      if (result.approvalFailed) {
        setRevokeApprovalFailure(result.approvalFailed);
        return;
      }
      setRevokeTarget(null);
      setRevokeApprovalFailure(null);
      toast.success(`Revoked ${vars.device.name}.`);
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to revoke device.");
    },
  });

  const retryApprovalMutation = useMutation({
    mutationFn: () => patchRemoteStatus({ require_device_approval: true }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      setRevokeTarget(null);
      setRevokeApprovalFailure(null);
      toast.success("Enabled approval for new devices on this computer.");
    },
    onError: err => {
      setRevokeApprovalFailure(
        err instanceof Error ? err.message : "Couldn't turn on device approval."
      );
    },
  });

  const handleToggleLocalDeviceScope = useCallback(
    (deviceId: string, scope: RemoteScope, next: boolean) => {
      const target = localDevices.find(item => item.id === deviceId);
      if (!target) return;
      const set = new Set<RemoteScope>(target.scopesOnLocal);
      if (next) set.add(scope);
      else set.delete(scope);
      localDevicePatchMutation.mutate({ deviceId, body: { scopes: [...set] } });
    },
    [localDevices, localDevicePatchMutation]
  );

  const handleDecideLocalPending = useCallback(
    (approvalId: string, approved: boolean) => {
      const target = remoteDevicesQuery.data?.devices.find(device =>
        device.approvals?.some(a => a.id === approvalId)
      );
      if (!target) return;
      localDevicePatchMutation.mutate({
        deviceId: target.id,
        body: { approval_id: approvalId, approved },
      });
    },
    [remoteDevicesQuery.data?.devices, localDevicePatchMutation]
  );

  return {
    remoteStatus: remoteStatusQuery.data,
    refetchRemoteStatus: remoteStatusQuery.refetch,
    localDevices,
    localPendingApprovals,
    revokeTarget,
    setRevokeTarget,
    revokeApprovalFailure,
    isRevoking: revokeLocalDeviceMutation.isPending || retryApprovalMutation.isPending,
    closeRevokeDialog: () => {
      setRevokeTarget(null);
      setRevokeApprovalFailure(null);
    },
    handleToggleRemoteEnabled: (next: boolean) => remotePatchMutation.mutate({ enabled: next }),
    handleToggleRequireApproval: (next: boolean) =>
      remotePatchMutation.mutate({ require_device_approval: next }),
    handleToggleLocalDeviceScope,
    handleApproveLocalPending: (approvalId: string) => handleDecideLocalPending(approvalId, true),
    handleDenyLocalPending: (approvalId: string) => handleDecideLocalPending(approvalId, false),
    handleConfirmRevokeDevice: (device: AccountDeviceFixture, enableApproval: boolean) =>
      revokeLocalDeviceMutation.mutate({ device, enableApproval }),
    handleRetryEnableApproval: () => retryApprovalMutation.mutate(),
  };
}
