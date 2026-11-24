import { useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import type {
  AccountDeviceFixture,
  AccountProvider,
  AccountStateFixture,
  PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-types";
import { patchRemoteStatus, unlinkRemoteHost } from "@/systems/computers/adapters/remote-host-api";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";
import { computersListOptions, remoteStatusOptions } from "@/systems/computers/lib/query-options";

import {
  AccountApiError,
  cancelAccountLogin,
  decideAccountApproval,
  logoutAccount,
  openAccountSignInStream,
  patchAccountDevice,
  revokeAccountDevice,
  startAccountLogin,
  type AccountLoginResponse,
} from "../adapters/account-api";
import {
  isAccountCloudUnreachableError,
  toAccountDeviceView,
  toAccountView,
  toPendingApprovalView,
} from "../lib/account-view";
import { accountKeys } from "../lib/query-keys";
import {
  accountApprovalsOptions,
  accountDevicesOptions,
  accountStatusOptions,
} from "../lib/query-options";

export interface UseAccountSettingsPageResult {
  account: AccountStateFixture | null;
  devices: AccountDeviceFixture[];
  currentDevice: AccountDeviceFixture | undefined;
  currentDeviceIsAdmin: boolean;
  approvals: PendingApprovalFixture[];
  downgradedNeedsProCount: number;
  localRemoteEnabled: boolean;
  requireDeviceApproval: boolean;
  isLoading: boolean;
  error: Error | null;
  loginAttempt: AccountLoginResponse | null;
  loginError: string | null;
  linkMismatchNotice: boolean;
  isStartingLogin: boolean;
  revokeTarget: AccountDeviceFixture | null;
  revokeApprovalFailure: string | null;
  isRevoking: boolean;
  signOutOpen: boolean;
  signOutError: string | null;
  isSigningOut: boolean;
  setRevokeTarget: (device: AccountDeviceFixture | null) => void;
  setSignOutOpen: (open: boolean) => void;
  handleToggleAdmin: (device: AccountDeviceFixture, nextAdmin: boolean) => void;
  handleConfirmRevoke: (device: AccountDeviceFixture, enableApproval: boolean) => void;
  handleRetryEnableApproval: () => void;
  handleCloseRevoke: () => void;
  handleApprove: (approvalId: string) => void;
  handleDeny: (approvalId: string) => void;
  handleConfirmSignOut: () => void;
  handleStartSignIn: (provider: AccountProvider) => void;
  handleCancelSignIn: () => void;
  handleUnlinkLocal: () => void;
  handleRetryCloud: () => void;
}

export function useAccountSettingsPage(): UseAccountSettingsPageResult {
  const queryClient = useQueryClient();

  const statusQuery = useQuery(accountStatusOptions());
  const signedIn = Boolean(statusQuery.data?.signed_in);
  const isSession = signedIn && statusQuery.data?.source === "session";

  const devicesQuery = useQuery(accountDevicesOptions(isSession));
  const approvalsQuery = useQuery(accountApprovalsOptions(isSession));
  const computersQuery = useQuery(computersListOptions(isSession));
  const remoteQuery = useQuery(remoteStatusOptions(signedIn));

  const [loginAttempt, setLoginAttempt] = useState<AccountLoginResponse | null>(null);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [linkMismatchNotice, setLinkMismatchNotice] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<AccountDeviceFixture | null>(null);
  const [revokeApprovalFailure, setRevokeApprovalFailure] = useState<string | null>(null);
  const [signOutOpen, setSignOutOpen] = useState(false);
  const [signOutError, setSignOutError] = useState<string | null>(null);

  // Subscribe to `/api/account/sign-in/events` while a login attempt is in progress.
  useEffect(() => {
    if (!loginAttempt) return;
    return openAccountSignInStream({
      onProgress: progress => {
        if (progress.state === "done" || progress.state === "succeeded") {
          setLoginAttempt(null);
          setLoginError(null);
          setLinkMismatchNotice(false);
          void queryClient.invalidateQueries({ queryKey: accountKeys.all });
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
          toast.success("Signed in to CompozyOS.");
        } else if (progress.state === "failed" || progress.state === "expired") {
          setLoginAttempt(null);
          setLoginError(
            progress.code
              ? `Sign-in did not complete (${progress.code}).`
              : "Sign-in did not complete."
          );
        } else if (progress.state === "canceled") {
          setLoginAttempt(null);
        }
      },
      onState: state => {
        if (state.state === "signed_in") {
          setLoginAttempt(null);
          setLoginError(null);
          setLinkMismatchNotice(false);
          void queryClient.invalidateQueries({ queryKey: accountKeys.all });
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
        }
      },
    });
  }, [loginAttempt, queryClient]);

  const cloudUnreachable =
    isAccountCloudUnreachableError(statusQuery.error) ||
    isAccountCloudUnreachableError(devicesQuery.error) ||
    isAccountCloudUnreachableError(approvalsQuery.error);

  const account = useMemo<AccountStateFixture | null>(() => {
    if (!statusQuery.data) return null;
    return toAccountView(statusQuery.data, { cloudUnreachable });
  }, [statusQuery.data, cloudUnreachable]);

  const devices = useMemo<AccountDeviceFixture[]>(() => {
    return (devicesQuery.data?.devices ?? []).map(device => toAccountDeviceView(device));
  }, [devicesQuery.data?.devices]);

  const currentDevice = useMemo(() => {
    const fromList = devices.find(device => device.thisDevice);
    if (fromList) return fromList;
    const summary = statusQuery.data?.device;
    if (!summary) return undefined;
    return {
      id: summary.id,
      name: summary.name,
      kind: "desktop" as const,
      admin: summary.admin,
      thisDevice: true,
      lastSeenAt: "",
      lastSeenLabel: "now",
      keyFingerprint: "",
      scopesOnLocal: [],
    };
  }, [devices, statusQuery.data?.device]);

  const computerNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const computer of computersQuery.data?.computers ?? []) {
      map.set(computer.id, computer.name);
    }
    return map;
  }, [computersQuery.data?.computers]);

  const approvals = useMemo<PendingApprovalFixture[]>(() => {
    return (approvalsQuery.data?.approvals ?? []).map(approval =>
      toPendingApprovalView(approval, computerNameById)
    );
  }, [approvalsQuery.data?.approvals, computerNameById]);

  const downgradedNeedsProCount = useMemo(
    () =>
      (computersQuery.data?.computers ?? []).filter(computer => computer.status === "needs_pro")
        .length,
    [computersQuery.data?.computers]
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

  const signOutMutation = useMutation({
    mutationFn: () => logoutAccount({ unlink: Boolean(account?.installationLinked) }),
    onSuccess: () => {
      setSignOutOpen(false);
      setSignOutError(null);
      void queryClient.invalidateQueries({ queryKey: accountKeys.all });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      toast.message("Signed out of CompozyOS.");
    },
    onError: err => {
      setSignOutError(err instanceof Error ? err.message : "Failed to sign out.");
    },
  });

  const startSignInMutation = useMutation({
    mutationFn: (provider: AccountProvider) =>
      startAccountLogin({ provider, marketing_opt_in: false }),
    onSuccess: response => {
      setLoginAttempt(response);
      setLoginError(null);
      setLinkMismatchNotice(false);
    },
    onError: err => {
      if (
        err instanceof AccountApiError &&
        (err.code === "link_account_mismatch" || err.status === 409)
      ) {
        setLinkMismatchNotice(true);
        return;
      }
      setLoginError(err instanceof Error ? err.message : "Failed to start sign-in.");
    },
  });

  const unlinkMutation = useMutation({
    mutationFn: () => unlinkRemoteHost(),
    onSuccess: () => {
      setLinkMismatchNotice(false);
      void queryClient.invalidateQueries({ queryKey: accountKeys.all });
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      toast.success("Unlinked this computer from remote access.");
    },
    onError: err => {
      toast.error(err instanceof Error ? err.message : "Failed to unlink this computer.");
    },
  });

  const handleCancelSignIn = useCallback(() => {
    const attemptId = loginAttempt?.attempt_id;
    setLoginAttempt(null);
    void cancelAccountLogin(attemptId).catch(() => undefined);
  }, [loginAttempt?.attempt_id]);

  const handleCloseRevoke = useCallback(() => {
    setRevokeTarget(null);
    setRevokeApprovalFailure(null);
  }, []);

  const handleRetryCloud = useCallback(() => {
    void Promise.all([
      statusQuery.refetch(),
      devicesQuery.refetch(),
      approvalsQuery.refetch(),
      remoteQuery.refetch(),
    ]);
  }, [statusQuery, devicesQuery, approvalsQuery, remoteQuery]);

  const hardError =
    statusQuery.error && !isAccountCloudUnreachableError(statusQuery.error)
      ? statusQuery.error
      : null;

  return {
    account,
    devices,
    currentDevice,
    currentDeviceIsAdmin: Boolean(currentDevice?.admin),
    approvals,
    downgradedNeedsProCount,
    localRemoteEnabled: Boolean(remoteQuery.data?.enabled),
    requireDeviceApproval: Boolean(remoteQuery.data?.require_device_approval),
    isLoading: statusQuery.isLoading,
    error: hardError,
    loginAttempt,
    loginError,
    linkMismatchNotice,
    isStartingLogin: startSignInMutation.isPending,
    revokeTarget,
    revokeApprovalFailure,
    isRevoking: revokeMutation.isPending || retryApprovalMutation.isPending,
    signOutOpen,
    signOutError,
    isSigningOut: signOutMutation.isPending,
    setRevokeTarget,
    setSignOutOpen: open => {
      setSignOutOpen(open);
      if (!open) setSignOutError(null);
    },
    handleToggleAdmin: (device, nextAdmin) =>
      toggleAdminMutation.mutate({ id: device.id, admin: nextAdmin }),
    handleConfirmRevoke: (device, enableApproval) =>
      revokeMutation.mutate({ device, enableApproval }),
    handleRetryEnableApproval: () => retryApprovalMutation.mutate(),
    handleCloseRevoke,
    handleApprove: approvalId => decideApprovalMutation.mutate({ id: approvalId, approve: true }),
    handleDeny: approvalId => decideApprovalMutation.mutate({ id: approvalId, approve: false }),
    handleConfirmSignOut: () => signOutMutation.mutate(),
    handleStartSignIn: provider => startSignInMutation.mutate(provider),
    handleCancelSignIn,
    handleUnlinkLocal: () => unlinkMutation.mutate(),
    handleRetryCloud,
  };
}
