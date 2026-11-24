import { useCallback, useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { toast } from "@compozy/ui";

import {
  accountKeys,
  accountStatusOptions,
  isAccountCloudUnreachableError,
  patchAccountDevice,
  toAccountView,
} from "@/systems/account";

import {
  addAccountComputer,
  fetchComputerSetupStatus,
  openComputersStream,
  patchComputer,
  removeComputer,
  removeComputerRoute,
  startComputerUpdate,
} from "../adapters/computers-api";
import type { ComputerDetailsTabId } from "../components/computer-details-panel";
import {
  isComputersCloudUnreachableError,
  toAvailableComputerView,
  toComputerView,
  toLocalComputerView,
} from "../lib/computer-view";
import { computerRegistryKeys } from "../lib/query-keys";
import { availableComputersOptions } from "../lib/query-options";
import { buildLiveUpdateSteps, toLiveUpdateState } from "../lib/ssh-setup-view";
import type {
  AccountStateFixture,
  AccountUnaddedComputerFixture,
  ComputerFixture,
  LocalComputerFixture,
  RemoteScope,
} from "../mocks/remote-computers-types";
import type { ComputerSetupStatusPayload } from "../types";
import { useComputers } from "./use-computers";
import { useLiveAddComputer } from "./use-live-add-computer";
import { useLocalRemoteControls } from "./use-local-remote-controls";

const FALLBACK_ACCOUNT: AccountStateFixture = {
  signedIn: false,
  accountId: "signed_out",
  name: "Account",
  email: "",
  provider: null,
  source: "session",
  tokenName: null,
  marketingOptIn: false,
  installationLinked: false,
  plan: "free",
  remoteComputersUsed: 0,
  remoteComputersLimit: 1,
  cloudRoutesUsed: 0,
  cloudRoutesLimit: 0,
  paymentIssue: false,
  cancelAtPeriodEnd: false,
  renewsAt: null,
  endsAt: null,
  formerSubscriber: false,
  cloudUnreachable: false,
  hostedRouteGraceDaysLeft: null,
};

export function useComputersSettingsPage() {
  const queryClient = useQueryClient();

  const accountQuery = useQuery(accountStatusOptions());
  const signedIn = Boolean(accountQuery.data?.signed_in);
  const currentDeviceId = accountQuery.data?.device?.id ?? null;
  const currentDeviceIsAdmin = Boolean(accountQuery.data?.device?.admin);
  const currentDeviceName = accountQuery.data?.device?.name ?? "This device";

  const computersQuery = useComputers({ enabled: signedIn, stream: signedIn });
  const availableQuery = useQuery(availableComputersOptions(signedIn));
  const localRemote = useLocalRemoteControls({ signedIn, currentDeviceId });

  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [selectedComputerId, setSelectedComputerId] = useState<string | null>(null);
  const [detailsTab, setDetailsTab] = useState<ComputerDetailsTabId>("status");
  const [removeTarget, setRemoveTarget] = useState<ComputerFixture | null>(null);
  const [renameTarget, setRenameTarget] = useState<ComputerFixture | null>(null);
  const [disableAgentsOpen, setDisableAgentsOpen] = useState(false);
  const [uninstallWarning, setUninstallWarning] = useState<string | null>(null);
  const [addingAccountId, setAddingAccountId] = useState<string | null>(null);
  const [localRunAgentsOverride, setLocalRunAgentsOverride] = useState<boolean | null>(null);
  const [updateSetupId, setUpdateSetupId] = useState<string | null>(null);
  const [updateStatus, setUpdateStatus] = useState<ComputerSetupStatusPayload | null>(null);

  const cloudUnreachable =
    isAccountCloudUnreachableError(accountQuery.error) ||
    isComputersCloudUnreachableError(computersQuery.error) ||
    isComputersCloudUnreachableError(availableQuery.error);

  const account = useMemo<AccountStateFixture>(() => {
    if (!accountQuery.data) {
      return { ...FALLBACK_ACCOUNT, cloudUnreachable };
    }
    return toAccountView(accountQuery.data, { cloudUnreachable });
  }, [accountQuery.data, cloudUnreachable]);

  const computers = useMemo<ComputerFixture[]>(
    () => computersQuery.computers.map(toComputerView),
    [computersQuery.computers]
  );

  const local = useMemo<LocalComputerFixture>(() => {
    const view = toLocalComputerView(computersQuery.local, {
      name: accountQuery.data?.device?.name ?? "This computer",
      remote: localRemote.remoteStatus,
    });
    return localRunAgentsOverride !== null
      ? { ...view, localRunAgents: localRunAgentsOverride }
      : view;
  }, [
    computersQuery.local,
    accountQuery.data?.device?.name,
    localRemote.remoteStatus,
    localRunAgentsOverride,
  ]);

  const unaddedComputers = useMemo<AccountUnaddedComputerFixture[]>(() => {
    const items: AccountUnaddedComputerFixture[] = [];
    for (const item of availableQuery.data?.computers ?? []) {
      if (!item.added) {
        items.push(toAvailableComputerView(item));
      }
    }
    return items;
  }, [availableQuery.data?.computers]);

  const selectedComputer = useMemo(
    () => computers.find(item => item.id === selectedComputerId) ?? null,
    [computers, selectedComputerId]
  );

  const openDetails = useCallback(
    (computer: ComputerFixture, tab: ComputerDetailsTabId = "status") => {
      setSelectedComputerId(computer.id);
      setDetailsTab(tab);
      setUpdateSetupId(null);
      setUpdateStatus(null);
    },
    []
  );

  const addComputer = useLiveAddComputer({
    open: addDialogOpen,
    signedIn,
    addedComputers: computersQuery.computers,
    onOpenModal: () => setAddDialogOpen(true),
    onRequestRemoveById: computerId => {
      const target = computers.find(item => item.id === computerId) ?? null;
      setAddDialogOpen(false);
      if (target) setRemoveTarget(target);
    },
  });

  useEffect(() => {
    if (!updateSetupId) return;
    return openComputersStream({
      onComputerSetup: change => {
        if (change.setup_id !== updateSetupId) return;
        setUpdateStatus(prev => ({
          setup_id: updateSetupId,
          computer_id: change.computer_id ?? prev?.computer_id ?? null,
          state: change.state ?? prev?.state ?? "running",
          step: change.step ?? prev?.step ?? "verify",
          prompt: change.prompt ?? null,
          error: change.error ?? null,
        }));
        if (change.state === "done") {
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
        }
      },
    });
  }, [updateSetupId, queryClient]);

  const toggleEnabledMutation = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      patchComputer(id, { enabled }),
    onSuccess: (_data, vars) => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      const target = computers.find(item => item.id === vars.id);
      toast.message(
        vars.enabled
          ? `Turned on ${target?.name ?? "computer"}.`
          : `Turned off ${target?.name ?? "computer"}.`
      );
    },
    onError: err => toast.error(err instanceof Error ? err.message : "Failed to update computer."),
  });

  const renameMutation = useMutation({
    mutationFn: ({ id, name }: { id: string; name: string }) => patchComputer(id, { name }),
    onSuccess: (_data, vars) => {
      setRenameTarget(null);
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      toast.success(`Renamed computer to ${vars.name}.`);
    },
    onError: err => toast.error(err instanceof Error ? err.message : "Failed to rename computer."),
  });

  const removeMutation = useMutation({
    mutationFn: ({
      computer,
      unlink,
      uninstall,
    }: {
      computer: ComputerFixture;
      unlink: boolean;
      uninstall: boolean;
    }) => removeComputer(computer.id, { unlink, uninstall }),
    onSuccess: (response, vars) => {
      setRemoveTarget(null);
      if (selectedComputerId === vars.computer.id) setSelectedComputerId(null);
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });

      if (response.uninstall === "failed") {
        setUninstallWarning(
          `Removed ${vars.computer.name} locally, but uninstalling over SSH didn't finish. Run "compozy uninstall" on ${vars.computer.sshTarget ?? vars.computer.name} to finish cleanup.`
        );
        return;
      }
      toast.success(`Removed ${vars.computer.name}.`);
    },
    onError: err => toast.error(err instanceof Error ? err.message : "Failed to remove computer."),
  });

  const removeRouteMutation = useMutation({
    mutationFn: ({ computerId, routeId }: { computerId: string; routeId: string }) =>
      removeComputerRoute(computerId, routeId),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      toast.message("Removed learned route.");
    },
    onError: err => toast.error(err instanceof Error ? err.message : "Failed to remove route."),
  });

  const addAccountMutation = useMutation({
    mutationFn: (computer: AccountUnaddedComputerFixture) =>
      addAccountComputer(computer.id, computer.name),
    onMutate: computer => setAddingAccountId(computer.id),
    onSettled: () => setAddingAccountId(null),
    onSuccess: (_data, computer) => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
      void queryClient.invalidateQueries({ queryKey: accountKeys.status() });
      toast.success(`Added ${computer.name} to your computers.`);
    },
    onError: err => toast.error(err instanceof Error ? err.message : "Failed to add computer."),
  });

  const handleToggleComputerScope = useCallback(
    (computer: ComputerFixture, scope: RemoteScope, next: boolean) => {
      if (!currentDeviceId) return;
      const set = new Set<RemoteScope>(computer.scopes);
      if (next) set.add(scope);
      else set.delete(scope);
      void patchAccountDevice(currentDeviceId, { scopes: { [computer.id]: [...set] } })
        .then(() => {
          void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
          toast.message(
            next
              ? `Allowed ${scope} on ${computer.name}.`
              : `Turned off ${scope} on ${computer.name}.`
          );
        })
        .catch(err =>
          toast.error(err instanceof Error ? err.message : "Failed to update permissions.")
        );
    },
    [currentDeviceId, queryClient]
  );

  const handleStartUpdate = useCallback(async (computer: ComputerFixture) => {
    try {
      const started = await startComputerUpdate(computer.id);
      setUpdateSetupId(started.setup_id);
      setUpdateStatus({
        setup_id: started.setup_id,
        computer_id: computer.id,
        state: "running",
        step: "verify",
        prompt: null,
        error: null,
      });
      const polled = await fetchComputerSetupStatus(started.setup_id).catch(() => null);
      if (polled) setUpdateStatus(polled);
    } catch (err) {
      setUpdateStatus({
        setup_id: "error",
        computer_id: computer.id,
        state: "failed",
        step: "verify",
        prompt: null,
        error: { error: err instanceof Error ? err.message : "Failed to start update." },
      });
    }
  }, []);

  const handleRequestToggleRunAgents = useCallback(
    (next: boolean) => {
      if (!next && local.activeLocalSessions > 0) {
        setDisableAgentsOpen(true);
        return;
      }
      setLocalRunAgentsOverride(next);
      toast.message(
        next ? "This computer can now run agents." : "Stopped starting new agents on this computer."
      );
    },
    [local.activeLocalSessions]
  );

  const handleRetryCloud = useCallback(() => {
    void Promise.all([
      accountQuery.refetch(),
      computersQuery.refetch(),
      availableQuery.refetch(),
      localRemote.refetchRemoteStatus(),
    ]);
  }, [accountQuery, computersQuery, availableQuery, localRemote]);

  const hardError =
    computersQuery.error && !isComputersCloudUnreachableError(computersQuery.error)
      ? computersQuery.error
      : null;

  return {
    account,
    local,
    computers,
    unaddedComputers,
    localDevices: localRemote.localDevices,
    localPendingApprovals: localRemote.localPendingApprovals,
    currentDeviceIsAdmin,
    currentDeviceName,
    cloudUnreachable,
    uninstallWarning,
    setUninstallWarning,
    isLoading: accountQuery.isLoading || (signedIn && computersQuery.isLoading),
    error: hardError,
    addDialogOpen,
    setAddDialogOpen: (open: boolean) => {
      setAddDialogOpen(open);
      if (!open) addComputer.resetSetup();
    },
    addComputer,
    selectedComputer,
    detailsTab,
    openDetails,
    closeDetails: () => setSelectedComputerId(null),
    removeTarget,
    setRemoveTarget,
    isRemoving: removeMutation.isPending,
    renameTarget,
    setRenameTarget,
    isRenaming: renameMutation.isPending,
    revokeTarget: localRemote.revokeTarget,
    setRevokeTarget: localRemote.setRevokeTarget,
    revokeApprovalFailure: localRemote.revokeApprovalFailure,
    isRevoking: localRemote.isRevoking,
    closeRevokeDialog: localRemote.closeRevokeDialog,
    disableAgentsOpen,
    setDisableAgentsOpen,
    addingAccountId,
    liveUpdateState: toLiveUpdateState(updateStatus),
    liveUpdateSteps: selectedComputer
      ? buildLiveUpdateSteps(updateStatus, selectedComputer.name)
      : undefined,
    handleRequestToggleRunAgents,
    handleConfirmDisableAgents: () => {
      setDisableAgentsOpen(false);
      setLocalRunAgentsOverride(false);
      toast.message("Stopped starting new agents on this computer.");
    },
    handleToggleRemoteEnabled: localRemote.handleToggleRemoteEnabled,
    handleToggleRequireApproval: localRemote.handleToggleRequireApproval,
    handleToggleLocalDeviceScope: localRemote.handleToggleLocalDeviceScope,
    handleApproveLocalPending: localRemote.handleApproveLocalPending,
    handleDenyLocalPending: localRemote.handleDenyLocalPending,
    handleConfirmRevokeDevice: localRemote.handleConfirmRevokeDevice,
    handleRetryEnableApproval: localRemote.handleRetryEnableApproval,
    handleToggleEnabled: (computer: ComputerFixture, nextEnabled: boolean) =>
      toggleEnabledMutation.mutate({ id: computer.id, enabled: nextEnabled }),
    handleConfirmRename: (computer: ComputerFixture, nextName: string) =>
      renameMutation.mutate({ id: computer.id, name: nextName }),
    handleConfirmRemove: (
      computer: ComputerFixture,
      options: { unlink: boolean; uninstall: boolean }
    ) => removeMutation.mutate({ computer, ...options }),
    handleRemoveRoute: (computer: ComputerFixture, routeId: string) =>
      removeRouteMutation.mutate({ computerId: computer.id, routeId }),
    handleAddAccountComputer: (computer: AccountUnaddedComputerFixture) =>
      addAccountMutation.mutate(computer),
    handleRetryConnection: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
    },
    handleToggleComputerScope,
    handleStartUpdate,
    handleCheckVersion: () => {
      void queryClient.invalidateQueries({ queryKey: computerRegistryKeys.all });
    },
    handleRetryCloud,
  };
}
