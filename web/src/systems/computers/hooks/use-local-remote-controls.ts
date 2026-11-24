import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import {
  AccountApiError,
  accountApprovalsOptions,
  accountDevicesOptions,
  accountKeys,
  decideAccountApproval,
  openAccountStream,
  type AccountDevicePayload,
} from "@/systems/account";

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

const HOSTED_ROUTE_POLL_INTERVAL_MS = 1_500;

export function useLocalRemoteControls({
  signedIn,
  currentDeviceId,
  currentDeviceIsAdmin = false,
}: {
  signedIn: boolean;
  currentDeviceId: string | null;
  currentDeviceIsAdmin?: boolean;
}) {
  const queryClient = useQueryClient();

  const remoteStatusQuery = useQuery(remoteStatusOptions(signedIn));
  const remoteEnabled = Boolean(remoteStatusQuery.data?.enabled);
  const remoteDevicesQuery = useQuery(remoteDevicesOptions(signedIn && remoteEnabled));
  const accountApprovalsQuery = useQuery(accountApprovalsOptions(signedIn && remoteEnabled));
  const accountDevicesQuery = useQuery(accountDevicesOptions(signedIn && remoteEnabled));

  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(null);
  const [revokeApprovalFailure, setRevokeApprovalFailure] = useState<string | null>(null);

  // Poll `/api/remote` while Hosted route is starting or retrying until it reaches `ready`.
  const hostedRouteEnabled = Boolean(remoteStatusQuery.data?.cloud_route?.enabled);
  const hostedRouteState = remoteStatusQuery.data?.cloud_route?.state;
  const shouldPollHostedRoute =
    signedIn && remoteEnabled && hostedRouteEnabled && hostedRouteState !== "ready";

  useEffect(() => {
    if (!shouldPollHostedRoute) return;
    const timer = setInterval(() => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
    }, HOSTED_ROUTE_POLL_INTERVAL_MS);
    return () => clearInterval(timer);
  }, [queryClient, shouldPollHostedRoute]);

  // Keep pending browser/device approvals and local device list live via `/api/account/stream`.
  useEffect(() => {
    if (!signedIn || !remoteEnabled) return;
    return openAccountStream({
      onAccountChanged: () => {
        void queryClient.invalidateQueries({ queryKey: accountKeys.approvals() });
        void queryClient.invalidateQueries({ queryKey: accountKeys.devices() });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      },
      onReset: () => {
        void queryClient.invalidateQueries({ queryKey: accountKeys.approvals() });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      },
    });
  }, [queryClient, remoteEnabled, signedIn]);

  const accountDevicesById = new Map<string, AccountDevicePayload>(
    (accountDevicesQuery.data?.devices ?? []).map(device => [device.id, device])
  );

  const localDevices: AccountDeviceFixture[] = (remoteDevicesQuery.data?.devices ?? []).map(
    device => toRemoteDeviceView(device, currentDeviceId, accountDevicesById)
  );

  const localPendingApprovals: PendingApprovalFixture[] = extractLocalPendingApprovals(
    remoteDevicesQuery.data?.devices ?? [],
    {
      accountApprovals: accountApprovalsQuery.data?.approvals,
      accountDevicesById,
      localComputerId: remoteStatusQuery.data?.computer_id,
      currentDeviceIsAdmin,
    }
  );

  const remotePatchMutation = useMutation({
    mutationFn: (body: Parameters<typeof patchRemoteStatus>[0]) => patchRemoteStatus(body),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
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
      void queryClient.invalidateQueries({ queryKey: accountKeys.approvals() });
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to update device.");
    },
  });

  const decideApprovalMutation = useMutation({
    mutationFn: async ({ approvalId, approved }: { approvalId: string; approved: boolean }) => {
      const localDevice = remoteDevicesQuery.data?.devices.find(device =>
        device.approvals?.some(approval => approval.id === approvalId)
      );
      const hasAccountApproval = (accountApprovalsQuery.data?.approvals ?? []).some(
        approval => approval.id === approvalId
      );

      if (currentDeviceIsAdmin && (hasAccountApproval || !localDevice)) {
        try {
          await decideAccountApproval(approvalId, approved);
          return;
        } catch (err) {
          const canFallbackLocally =
            localDevice !== undefined &&
            err instanceof AccountApiError &&
            (err.code === "approver_not_approved_on_computer" || err.status === 404);
          if (!canFallbackLocally) throw err;
        }
      }

      if (!localDevice) {
        await decideAccountApproval(approvalId, approved);
        return;
      }

      await patchRemoteDevice(localDevice.id, {
        approval_id: approvalId,
        approved,
      });
    },
    onSuccess: (_data, vars) => {
      void queryClient.invalidateQueries({ queryKey: accountKeys.approvals() });
      void queryClient.invalidateQueries({ queryKey: accountKeys.devices() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      toast.success(vars.approved ? "Device approved." : "Request denied.");
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to decide device approval.");
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

  const handleToggleLocalDeviceScope = (deviceId: string, scope: RemoteScope, next: boolean) => {
    const target = localDevices.find(item => item.id === deviceId);
    if (!target) return;
    const set = new Set<RemoteScope>(target.scopesOnLocal);
    if (next) set.add(scope);
    else set.delete(scope);
    localDevicePatchMutation.mutate({ deviceId, body: { scopes: [...set] } });
  };

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
    handleToggleHostedRoute: (next: boolean) => remotePatchMutation.mutate({ cloud_route: next }),
    handleToggleRequireApproval: (next: boolean) =>
      remotePatchMutation.mutate({ require_device_approval: next }),
    handleToggleLocalDeviceScope,
    handleApproveLocalPending: (approvalId: string) =>
      decideApprovalMutation.mutate({ approvalId, approved: true }),
    handleDenyLocalPending: (approvalId: string) =>
      decideApprovalMutation.mutate({ approvalId, approved: false }),
    handleConfirmRevokeDevice: (device: AccountDeviceFixture, enableApproval: boolean) =>
      revokeLocalDeviceMutation.mutate({ device, enableApproval }),
    handleRetryEnableApproval: () => retryApprovalMutation.mutate(),
  };
}
