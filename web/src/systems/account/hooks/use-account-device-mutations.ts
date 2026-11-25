import { useCallback, useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import { onAccountScopeEvicted } from "@/lib/computer-scope";

import { patchRemoteStatus } from "@/systems/computers/adapters/remote-host-api";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";
import type { AccountDeviceFixture } from "@/systems/computers/mocks/remote-computers-types";

import {
  decideAccountApproval,
  patchAccountDevice,
  revokeAccountDevice,
} from "../adapters/account-api";
import type { AccountSignInNoticeView } from "../lib/account-view";
import { accountKeys } from "../lib/query-keys";

export function useAccountDeviceMutations(devices: readonly AccountDeviceFixture[]) {
  const queryClient = useQueryClient();
  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(null);
  const [revokeApprovalFailure, setRevokeApprovalFailure] = useState<string | null>(null);

  useEffect(
    () =>
      onAccountScopeEvicted(() => {
        setRevokeTarget(null);
        setRevokeApprovalFailure(null);
      }),
    []
  );

  const toggleAdminMutation = useMutation({
    mutationFn: ({ id, admin }: { id: string; admin: boolean }) =>
      patchAccountDevice(id, { admin }),
    onSuccess: (_data, vars) => {
      void queryClient.invalidateQueries({ queryKey: accountKeys.devices() });
      const target = devices.find(device => device.id === vars.id);
      toast.message(
        vars.admin
          ? `${target?.name ?? "Device"} is now trusted for admin.`
          : `Removed admin trust from ${target?.name ?? "device"}.`
      );
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to update device.");
    },
  });

  const revokeMutation = useMutation({
    mutationFn: async ({
      device,
      enableApproval,
    }: {
      device: AccountDeviceFixture;
      enableApproval: boolean;
    }) => {
      await revokeAccountDevice(device.id);
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
      void queryClient.invalidateQueries({ queryKey: accountKeys.devices() });
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      if (result.approvalFailed) {
        setRevokeApprovalFailure(result.approvalFailed);
        return;
      }
      setRevokeTarget(null);
      setRevokeApprovalFailure(null);
      toast.success(
        vars.enableApproval
          ? `Revoked ${vars.device.name} and enabled approval for new devices.`
          : `Revoked ${vars.device.name}.`
      );
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

  const decideApprovalMutation = useMutation({
    mutationFn: ({ id, approve }: { id: string; approve: boolean }) =>
      decideAccountApproval(id, approve),
    onSuccess: (_data, vars) => {
      void queryClient.invalidateQueries({ queryKey: accountKeys.approvals() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
      toast.success(vars.approve ? "Approval decision sent." : "Request denied.");
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to send decision.");
    },
  });

  const handleCloseRevoke = useCallback(() => {
    setRevokeTarget(null);
    setRevokeApprovalFailure(null);
  }, []);

  const handleRevokeSignInNotice = useCallback(
    (notice: AccountSignInNoticeView) => {
      const matched = devices.find(device => device.id === notice.deviceId);
      if (matched) {
        setRevokeTarget(matched);
        return;
      }
      setRevokeTarget({
        id: notice.deviceId,
        name: notice.name,
        kind: notice.kind,
        admin: false,
        thisDevice: false,
        lastSeenAt: notice.at,
        lastSeenLabel: notice.whenLabel,
        keyFingerprint: "",
        scopesOnLocal: [],
      });
    },
    [devices]
  );

  return {
    revokeTarget,
    setRevokeTarget,
    revokeApprovalFailure,
    isRevoking: revokeMutation.isPending || retryApprovalMutation.isPending,
    handleToggleAdmin: (device: AccountDeviceFixture, nextAdmin: boolean) =>
      toggleAdminMutation.mutate({ id: device.id, admin: nextAdmin }),
    handleConfirmRevoke: (device: AccountDeviceFixture, enableApproval: boolean) =>
      revokeMutation.mutate({ device, enableApproval }),
    handleRetryEnableApproval: () => retryApprovalMutation.mutate(),
    handleCloseRevoke,
    handleApprove: (approvalId: string) =>
      decideApprovalMutation.mutate({ id: approvalId, approve: true }),
    handleDeny: (approvalId: string) =>
      decideApprovalMutation.mutate({ id: approvalId, approve: false }),
    handleRevokeSignInNotice,
  };
}
