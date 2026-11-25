import { useCallback, useEffect, useMemo, useState, useTransition } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "@tanstack/react-router";

import { toast } from "@compozy/ui";

import { evictAccountScopedState, onAccountScopeEvicted } from "@/lib/computer-scope";
import { unlinkRemoteHost } from "@/systems/computers/adapters/remote-host-api";
import { computerRegistryKeys } from "@/systems/computers/lib/query-keys";
import { computersListOptions, remoteStatusOptions } from "@/systems/computers/lib/query-options";
import type { CreatedAccountTokenFixture } from "@/systems/computers/mocks/remote-computers-fixtures-lane-a";
import type {
  AccountDeviceFixture,
  AccountProvider,
  AccountStateFixture,
  AccountTokenFixture,
  PendingApprovalFixture,
} from "@/systems/computers/mocks/remote-computers-types";

import {
  AccountApiError,
  cancelAccountLogin,
  logoutAccount,
  openAccountBillingPortal,
  openAccountSignInStream,
  openAccountStream,
  startAccountLogin,
  type AccountLoginResponse,
} from "../adapters/account-api";
import { useOptionalUpgradeSheetActions } from "./use-optional-upgrade-sheet-actions";
import {
  buildSignedOutAccountStatus,
  describeBillingApiError,
  isAccountCloudUnreachableError,
  toAccountDeviceView,
  toAccountSignInNoticeView,
  toAccountView,
  toPendingApprovalView,
  type AccountSignInNoticeView,
} from "../lib/account-view";
import { openBillingBrowserWindow } from "../lib/billing-window";
import { accountKeys } from "../lib/query-keys";
import {
  accountApprovalsOptions,
  accountDevicesOptions,
  accountStatusOptions,
} from "../lib/query-options";
import { isAccountGraceActive } from "../lib/sign-in-gate";
import { useAccountDeviceMutations } from "./use-account-device-mutations";
import { useAccountLifecycleMutations } from "./use-account-lifecycle-mutations";
import { useUpgradeSheet, type UpgradeSheetController } from "./use-upgrade-sheet";

export interface UseAccountSettingsPageResult {
  account: AccountStateFixture | null;
  isGraceMode: boolean;
  devices: AccountDeviceFixture[];
  currentDevice: AccountDeviceFixture | undefined;
  currentDeviceIsAdmin: boolean;
  approvals: PendingApprovalFixture[];
  recentSignIns: AccountSignInNoticeView[];
  tokens: AccountTokenFixture[];
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
  signOutRequiresUnlink: boolean;
  signOutError: string | null;
  isSigningOut: boolean;
  isOpeningBillingPortal: boolean;
  isUpdatingEmail: boolean;
  createTokenOpen: boolean;
  createTokenError: string | null;
  isCreatingToken: boolean;
  createdToken: CreatedAccountTokenFixture | null;
  revokingTokenId: string | null;
  isDismissingNotices: boolean;
  deleteOpen: boolean;
  deleteError: string | null;
  isDeletingAccount: boolean;
  fallbackUpgradeSheet: UpgradeSheetController | null;
  setRevokeTarget: (device: AccountDeviceFixture | null) => void;
  setSignOutOpen: (open: boolean) => void;
  setCreateTokenOpen: (open: boolean) => void;
  setCreatedToken: (token: CreatedAccountTokenFixture | null) => void;
  setDeleteOpen: (open: boolean) => void;
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
  handleOpenUpgrade: () => void;
  handleManageBilling: () => void;
  handleToggleMarketingOptIn: (next: boolean) => void;
  handleCreateToken: (name: string) => void;
  handleRevokeToken: (token: AccountTokenFixture) => void;
  handleRevokeSignInNotice: (notice: AccountSignInNoticeView) => void;
  handleDismissSignInNotices: () => void;
  handleConfirmDeleteAccount: () => void;
  handleGoToSignIn: () => void;
}

export function useAccountSettingsPage(): UseAccountSettingsPageResult {
  const queryClient = useQueryClient();
  const router = useRouter();
  const sharedUpgradeSheet = useOptionalUpgradeSheetActions();
  const localUpgradeSheet = useUpgradeSheet();

  const statusQuery = useQuery(accountStatusOptions());
  const signedIn = Boolean(statusQuery.data?.signed_in);
  const isSession = signedIn && statusQuery.data?.source === "session";
  const isGraceMode = isAccountGraceActive(statusQuery.data);

  const devicesQuery = useQuery(accountDevicesOptions(isSession));
  const approvalsQuery = useQuery(accountApprovalsOptions(isSession));
  const computersQuery = useQuery(computersListOptions(isSession));
  const remoteQuery = useQuery(remoteStatusOptions(signedIn));

  const lifecycle = useAccountLifecycleMutations({
    isSession,
    email: statusQuery.data?.email ?? "",
    gate: statusQuery.data?.gate,
  });

  const [loginAttempt, setLoginAttempt] = useState<AccountLoginResponse | null>(null);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [linkMismatchNotice, setLinkMismatchNotice] = useState(false);
  const [signOutOpen, setSignOutOpen] = useState(false);
  const [signOutRequiresUnlink, setSignOutRequiresUnlink] = useState(false);
  const [signOutError, setSignOutError] = useState<string | null>(null);

  useEffect(
    () =>
      onAccountScopeEvicted(() => {
        setLoginAttempt(null);
        setLoginError(null);
        setLinkMismatchNotice(false);
        setSignOutOpen(false);
        setSignOutRequiresUnlink(false);
        setSignOutError(null);
      }),
    []
  );

  useEffect(() => {
    if (!isSession) return;
    return openAccountStream({
      onAccountChanged: event => {
        if (event.evict_remote) {
          evictAccountScopedState(queryClient);
          return;
        }
        void queryClient.invalidateQueries({ queryKey: accountKeys.all });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteDevices() });
        void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.remoteStatus() });
      },
      onReset: () => {
        void queryClient.invalidateQueries({ queryKey: accountKeys.all });
      },
    });
  }, [isSession, queryClient]);

  useEffect(() => {
    if (!loginAttempt) return;
    return openAccountSignInStream({
      onProgress: progress => {
        if (progress.attempt_id && progress.attempt_id !== loginAttempt.attempt_id) {
          return;
        }
        if (
          progress.state === "signed_in" ||
          progress.state === "done" ||
          progress.state === "succeeded"
        ) {
          setLoginAttempt(null);
          setLoginError(null);
          setLinkMismatchNotice(false);
          evictAccountScopedState(queryClient);
          toast.success("Signed in to CompozyOS.");
        } else if (progress.state === "failed" || progress.state === "expired") {
          setLoginAttempt(null);
          if (progress.code === "installation_linked_to_other_account") {
            setLinkMismatchNotice(true);
            return;
          }
          setLoginError(
            progress.code === "account_login_expired"
              ? "Sign-in timed out. Try again."
              : progress.code
                ? `Sign-in did not complete (${progress.code}).`
                : "Sign-in did not complete."
          );
        } else if (progress.state === "cancelled" || progress.state === "canceled") {
          setLoginAttempt(null);
        }
      },
      onState: state => {
        if (state.state === "signed_in") {
          setLoginAttempt(null);
          setLoginError(null);
          setLinkMismatchNotice(false);
          evictAccountScopedState(queryClient);
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

  const devices = useMemo<AccountDeviceFixture[]>(
    () => (devicesQuery.data?.devices ?? []).map(device => toAccountDeviceView(device)),
    [devicesQuery.data?.devices]
  );

  const deviceMutations = useAccountDeviceMutations(devices);

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

  const approvals = useMemo<PendingApprovalFixture[]>(
    () =>
      (approvalsQuery.data?.approvals ?? []).map(approval =>
        toPendingApprovalView(approval, computerNameById)
      ),
    [approvalsQuery.data?.approvals, computerNameById]
  );

  const recentSignIns = useMemo<AccountSignInNoticeView[]>(
    () => (statusQuery.data?.recent_sign_ins ?? []).map(toAccountSignInNoticeView),
    [statusQuery.data?.recent_sign_ins]
  );

  const downgradedNeedsProCount = useMemo(
    () =>
      (computersQuery.data?.computers ?? []).filter(computer => computer.status === "needs_pro")
        .length,
    [computersQuery.data?.computers]
  );

  const signOutMutation = useMutation({
    mutationFn: () =>
      logoutAccount({ unlink: Boolean(account?.installationLinked || signOutRequiresUnlink) }),
    onSuccess: () => {
      const gate = statusQuery.data?.gate;
      const signedOutStatus = buildSignedOutAccountStatus(statusQuery.data, "signed_out");
      setSignOutOpen(false);
      setSignOutRequiresUnlink(false);
      setSignOutError(null);
      evictAccountScopedState(queryClient, signedOutStatus ? { seedStatus: signedOutStatus } : {});
      toast.message("Signed out of CompozyOS.");
      if (gate && gate !== "off") {
        void router.navigate({ to: "/sign-in", replace: true });
      }
    },
    onError: err => {
      if (err instanceof AccountApiError && err.code === "logout_requires_unlink") {
        setSignOutRequiresUnlink(true);
        setSignOutError("Signing out also unlinks this computer from remote access.");
        return;
      }
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
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
    },
    onError: err => {
      if (
        err instanceof AccountApiError &&
        (err.code === "installation_linked_to_other_account" ||
          err.code === "link_account_mismatch" ||
          err.status === 409)
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

  const [isOpeningBillingPortal, startOpeningBillingPortal] = useTransition();

  const handleOpenUpgrade = useCallback(() => {
    if (sharedUpgradeSheet) {
      sharedUpgradeSheet.openUpgradeSheet();
      return;
    }
    localUpgradeSheet.openUpgradeSheet();
  }, [sharedUpgradeSheet, localUpgradeSheet]);

  const handleManageBilling = () => {
    if (isOpeningBillingPortal) return;
    const tab = openBillingBrowserWindow();
    startOpeningBillingPortal(async () => {
      try {
        const result = await openAccountBillingPortal();
        tab.navigate(result.url);
      } catch (err) {
        tab.close();
        toast.error(describeBillingApiError(err, "Couldn't open billing portal. Try again."));
      }
    });
  };

  const handleCancelSignIn = useCallback(() => {
    const attemptId = loginAttempt?.attempt_id;
    setLoginAttempt(null);
    void cancelAccountLogin(attemptId).catch(() => undefined);
  }, [loginAttempt?.attempt_id]);

  const handleRetryCloud = useCallback(() => {
    void Promise.all([
      statusQuery.refetch(),
      devicesQuery.refetch(),
      approvalsQuery.refetch(),
      remoteQuery.refetch(),
    ]);
  }, [statusQuery, devicesQuery, approvalsQuery, remoteQuery]);

  const handleGoToSignIn = useCallback(() => {
    void router.navigate({ to: "/sign-in", search: { redirect: "/settings/account" } });
  }, [router]);

  const hardError =
    statusQuery.error && !isAccountCloudUnreachableError(statusQuery.error)
      ? statusQuery.error
      : null;

  return {
    account,
    isGraceMode,
    devices,
    currentDevice,
    currentDeviceIsAdmin: Boolean(currentDevice?.admin),
    approvals,
    recentSignIns,
    tokens: lifecycle.tokens,
    downgradedNeedsProCount,
    localRemoteEnabled: Boolean(remoteQuery.data?.enabled),
    requireDeviceApproval: Boolean(remoteQuery.data?.require_device_approval),
    isLoading: statusQuery.isLoading,
    error: hardError,
    loginAttempt,
    loginError,
    linkMismatchNotice,
    isStartingLogin: startSignInMutation.isPending,
    revokeTarget: deviceMutations.revokeTarget,
    revokeApprovalFailure: deviceMutations.revokeApprovalFailure,
    isRevoking: deviceMutations.isRevoking,
    signOutOpen,
    signOutRequiresUnlink: Boolean(account?.installationLinked || signOutRequiresUnlink),
    signOutError,
    isSigningOut: signOutMutation.isPending,
    isOpeningBillingPortal,
    isUpdatingEmail: lifecycle.isUpdatingEmail,
    createTokenOpen: lifecycle.createTokenOpen,
    createTokenError: lifecycle.createTokenError,
    isCreatingToken: lifecycle.isCreatingToken,
    createdToken: lifecycle.createdToken,
    revokingTokenId: lifecycle.revokingTokenId,
    isDismissingNotices: lifecycle.isDismissingNotices,
    deleteOpen: lifecycle.deleteOpen,
    deleteError: lifecycle.deleteError,
    isDeletingAccount: lifecycle.isDeletingAccount,
    fallbackUpgradeSheet: sharedUpgradeSheet ? null : localUpgradeSheet,
    setRevokeTarget: deviceMutations.setRevokeTarget,
    setSignOutOpen: open => {
      setSignOutOpen(open);
      if (!open) {
        setSignOutError(null);
        setSignOutRequiresUnlink(false);
      }
    },
    setCreateTokenOpen: lifecycle.setCreateTokenOpen,
    setCreatedToken: lifecycle.setCreatedToken,
    setDeleteOpen: lifecycle.setDeleteOpen,
    handleToggleAdmin: deviceMutations.handleToggleAdmin,
    handleConfirmRevoke: deviceMutations.handleConfirmRevoke,
    handleRetryEnableApproval: deviceMutations.handleRetryEnableApproval,
    handleCloseRevoke: deviceMutations.handleCloseRevoke,
    handleApprove: deviceMutations.handleApprove,
    handleDeny: deviceMutations.handleDeny,
    handleConfirmSignOut: () => signOutMutation.mutate(),
    handleStartSignIn: provider => startSignInMutation.mutate(provider),
    handleCancelSignIn,
    handleUnlinkLocal: () => unlinkMutation.mutate(),
    handleRetryCloud,
    handleOpenUpgrade,
    handleManageBilling,
    handleToggleMarketingOptIn: lifecycle.handleToggleMarketingOptIn,
    handleCreateToken: lifecycle.handleCreateToken,
    handleRevokeToken: lifecycle.handleRevokeToken,
    handleRevokeSignInNotice: deviceMutations.handleRevokeSignInNotice,
    handleDismissSignInNotices: lifecycle.handleDismissSignInNotices,
    handleConfirmDeleteAccount: lifecycle.handleConfirmDeleteAccount,
    handleGoToSignIn,
  };
}
