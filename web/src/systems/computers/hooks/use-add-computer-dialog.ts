import { useState } from "react";

import { toast } from "@compozy/ui";

import type { SshTargetDraft } from "../components/add-computer-ssh-picker";
import type {
  AccountUnaddedComputerFixture,
  SshHostSuggestionFixture,
} from "../mocks/remote-computers-fixtures";
import type {
  AddComputerMethod,
  AddComputerSshStage,
} from "../mocks/remote-computers-fixtures-lane-b";

const ACTIVE_SSH_STAGES = new Set<AddComputerSshStage>([
  "running",
  "trust_host",
  "password",
  "agent_secret",
  "cancel_confirm",
]);

function resolveSshTarget(
  draft: SshTargetDraft,
  sshSuggestions: readonly SshHostSuggestionFixture[]
) {
  const selectedSuggestion = sshSuggestions.find(
    item => item.alias === draft.host || item.hostname === draft.host
  );
  const resolvedHost = selectedSuggestion ? selectedSuggestion.hostname : draft.host;
  const resolvedUser = draft.user || selectedSuggestion?.user || "";
  const resolvedPort = draft.port && draft.port !== "22" ? `:${draft.port}` : "";
  const targetAddress = resolvedUser
    ? `${resolvedUser}@${resolvedHost}${resolvedPort}`
    : `${resolvedHost}${resolvedPort}`;
  const hostLabel = draft.name || draft.host || "staging-box";
  const pickerBlocked =
    !draft.host.trim() ||
    Boolean(selectedSuggestion?.alreadyAdded) ||
    Boolean(selectedSuggestion?.unsupportedPlatform);

  return { selectedSuggestion, targetAddress, hostLabel, pickerBlocked };
}

function nextSshStageOnStart(
  draft: SshTargetDraft,
  selectedSuggestion: SshHostSuggestionFixture | undefined,
  isFreePlan: boolean
): AddComputerSshStage | null {
  if (!draft.host.trim() || selectedSuggestion?.alreadyAdded) return null;
  if (selectedSuggestion?.unsupportedPlatform) return "unsupported";
  if (isFreePlan) return "plan_limit";
  return "running";
}

interface UseAddComputerDialogInput {
  isFreePlan: boolean;
  sshSuggestions: readonly SshHostSuggestionFixture[];
  initialMethod: AddComputerMethod;
  initialSshStage: AddComputerSshStage;
  initialHostDraft: SshTargetDraft;
  linkedCommitReached: boolean;
  onClose?: () => void;
}

export function useAddComputerDialog({
  isFreePlan,
  sshSuggestions,
  initialMethod,
  initialSshStage,
  initialHostDraft,
  linkedCommitReached,
  onClose,
}: UseAddComputerDialogInput) {
  const [method, setMethod] = useState<AddComputerMethod>(initialMethod);
  const [sshStage, setSshStage] = useState<AddComputerSshStage>(initialSshStage);
  const [draft, setDraft] = useState<SshTargetDraft>(initialHostDraft);
  const [cancelConfirmOpen, setCancelConfirmOpen] = useState(initialSshStage === "cancel_confirm");
  const [addedIds, setAddedIds] = useState<string[]>([]);

  const { selectedSuggestion, targetAddress, hostLabel, pickerBlocked } = resolveSshTarget(
    draft,
    sshSuggestions
  );
  const isSshProgressView = method === "ssh" && sshStage !== "picker";
  const isActivelyRunning = ACTIVE_SSH_STAGES.has(sshStage);

  const handleRequestCancel = () => {
    if (isSshProgressView && isActivelyRunning && !linkedCommitReached) {
      setCancelConfirmOpen(true);
      return;
    }
    onClose?.();
  };

  const handleStartSsh = () => {
    const nextStage = nextSshStageOnStart(draft, selectedSuggestion, isFreePlan);
    if (nextStage) setSshStage(nextStage);
  };

  const handleAddAccountItem = (item: AccountUnaddedComputerFixture) => {
    setAddedIds(current => [...current, item.id]);
    toast.success(`Added ${item.name} to your computers.`);
  };

  const handleRetryFailed = () => {
    setSshStage("running");
    toast.message("Resuming setup from Installing CompozyOS 0.8.0…");
  };

  return {
    method,
    setMethod,
    sshStage,
    setSshStage,
    draft,
    setDraft,
    cancelConfirmOpen,
    setCancelConfirmOpen,
    addedIds,
    targetAddress,
    hostLabel,
    isSshProgressView,
    pickerBlocked,
    handleRequestCancel,
    handleStartSsh,
    handleAddAccountItem,
    handleRetryFailed,
  };
}
