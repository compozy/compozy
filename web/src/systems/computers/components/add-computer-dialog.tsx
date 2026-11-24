import { useState } from "react";
import { Laptop, Link2, Terminal, UserCheck } from "@compozy/ui/icons";

import {
  cn,
  ConfirmDialog,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogHeader,
  RadioCardGroup,
  toast,
  type RadioCardGroupOption,
} from "@compozy/ui";

import { useAddComputerDialog } from "../hooks/use-add-computer-dialog";
import type { AddComputerLiveController } from "../hooks/use-live-add-computer";
import type {
  AccountUnaddedComputerFixture,
  SshHostSuggestionFixture,
} from "../mocks/remote-computers-fixtures";
import type {
  AddComputerMethod,
  AddComputerSshStage,
} from "../mocks/remote-computers-fixtures-lane-b";
import {
  AddComputerAccountPath,
  AddComputerCommandPath,
  AddComputerFooter,
} from "./add-computer-paths";
import { AddComputerSshPicker, type SshTargetDraft } from "./add-computer-ssh-picker";
import { AddComputerSshProgressView } from "./add-computer-ssh-progress";

export interface AddComputerDialogProps {
  open: boolean;
  accountEmail: string;
  isFreePlan?: boolean;
  sshSuggestions: readonly SshHostSuggestionFixture[];
  unaddedComputers: readonly AccountUnaddedComputerFixture[];
  initialMethod?: AddComputerMethod;
  initialSshStage?: AddComputerSshStage;
  initialHostDraft?: SshTargetDraft;
  defaultHostPickerOpen?: boolean;
  /** Shows an already-detected computer in the Link with a command watcher. */
  initialCommandDetected?: boolean;
  /** Simulates the post-link phase where Cancel becomes Remove (Rule 19). */
  linkedCommitReached?: boolean;
  /** Live controller for production Settings → Computers; omitted in prototype drafts. */
  live?: AddComputerLiveController;
  onClose?: () => void;
  onUpgrade?: () => void;
  onOpenComputer?: (computerName: string) => void;
}

const DEFAULT_DRAFT: SshTargetDraft = {
  host: "staging-box",
  user: "deploy",
  port: "22",
  name: "staging-box",
};

const ACTIVE_SSH_STAGES = new Set<AddComputerSshStage>([
  "running",
  "trust_host",
  "password",
  "agent_secret",
  "cancel_confirm",
]);

function resolveTarget(draft: SshTargetDraft, sshSuggestions: readonly SshHostSuggestionFixture[]) {
  const selectedSuggestion = sshSuggestions.find(
    item => item.alias === draft.host || item.hostname === draft.host
  );
  const resolvedHost = selectedSuggestion ? selectedSuggestion.hostname : draft.host;
  const resolvedUser = draft.user || selectedSuggestion?.user || "";
  const resolvedPort = draft.port && draft.port !== "22" ? `:${draft.port}` : "";
  const targetAddress = resolvedUser
    ? `${resolvedUser}@${resolvedHost}${resolvedPort}`
    : `${resolvedHost}${resolvedPort}`;
  const hostLabel = draft.name || draft.host || "computer";
  const pickerBlocked =
    !draft.host.trim() ||
    Boolean(selectedSuggestion?.alreadyAdded) ||
    Boolean(selectedSuggestion?.unsupportedPlatform);

  return { targetAddress, hostLabel, pickerBlocked };
}

function buildMethodOptions(unaddedCount: number): Array<RadioCardGroupOption<AddComputerMethod>> {
  const accountCountLabel =
    unaddedCount === 1 ? "1 computer ready to add" : `${unaddedCount} computers ready to add`;
  const options: Array<RadioCardGroupOption<AddComputerMethod>> = [
    {
      value: "ssh",
      title: "SSH",
      description: "Install and connect over SSH using your SSH config",
      icon: Terminal,
    },
  ];
  if (unaddedCount > 0) {
    options.push({
      value: "account",
      title: "On your account",
      description: accountCountLabel,
      icon: UserCheck,
    });
  }
  options.push({
    value: "command",
    title: "Link with a command",
    description: "Run one command on the other computer, including Windows",
    icon: Link2,
  });
  return options;
}

function buildLiveDialogView(
  live: AddComputerLiveController,
  sshSuggestions: readonly SshHostSuggestionFixture[],
  cancelConfirmOpen: boolean,
  setCancelConfirmOpen: (open: boolean) => void,
  setLocalSshStage: ReturnType<typeof useAddComputerDialog>["setSshStage"],
  onClose?: () => void
) {
  const target = resolveTarget(live.draft, sshSuggestions);
  const isSshProgressView = live.method === "ssh" && live.sshStage !== "picker";
  const selectMethod = (next: AddComputerMethod) => {
    live.setMethod(next);
    live.onBackToPicker();
  };
  return {
    method: live.method,
    draft: live.draft,
    sshStage: live.sshStage,
    isSshProgressView,
    resolvedLinkedCommit: live.linkedCommitReached,
    addedIds: live.addedIds,
    commandDetected: Boolean(live.detectedCommandComputer),
    targetAddress: target.targetAddress,
    hostLabel: target.hostLabel,
    pickerBlocked: target.pickerBlocked,
    cancelConfirmOpen,
    setCancelConfirmOpen,
    setDraft: live.setDraft,
    setSshStage: setLocalSshStage,
    onAddAccountComputer: live.onAddAccountComputer,
    onStartSsh: live.onStartSsh,
    onRetry: live.onRetrySetup,
    onRemoveLinked: live.onRemoveLinked,
    selectMethod,
    switchToCommand: () => selectMethod("command"),
    handleRequestCancel: () => {
      if (isSshProgressView && ACTIVE_SSH_STAGES.has(live.sshStage) && !live.linkedCommitReached) {
        setCancelConfirmOpen(true);
        return;
      }
      onClose?.();
    },
    handleConfirmCancel: () => {
      setCancelConfirmOpen(false);
      live.onConfirmCancel();
    },
  };
}

function buildLocalDialogView(
  localState: ReturnType<typeof useAddComputerDialog>,
  initialCommandDetected: boolean,
  linkedCommitReached: boolean
) {
  const selectMethod = (next: AddComputerMethod) => {
    localState.setMethod(next);
    localState.setSshStage("picker");
  };
  return {
    method: localState.method,
    draft: localState.draft,
    sshStage: localState.sshStage,
    isSshProgressView: localState.method === "ssh" && localState.sshStage !== "picker",
    resolvedLinkedCommit: linkedCommitReached,
    addedIds: localState.addedIds,
    commandDetected: initialCommandDetected,
    targetAddress: localState.targetAddress,
    hostLabel: localState.hostLabel,
    pickerBlocked: localState.pickerBlocked,
    cancelConfirmOpen: localState.cancelConfirmOpen,
    setCancelConfirmOpen: localState.setCancelConfirmOpen,
    setDraft: localState.setDraft,
    setSshStage: localState.setSshStage,
    onAddAccountComputer: localState.handleAddAccountItem,
    onStartSsh: localState.handleStartSsh,
    onRetry: localState.handleRetryFailed,
    onRemoveLinked: () => toast.message(`Opens Remove dialog for ${localState.hostLabel}`),
    selectMethod,
    switchToCommand: () => selectMethod("command"),
    handleRequestCancel: localState.handleRequestCancel,
    handleConfirmCancel: () => {
      localState.setCancelConfirmOpen(false);
      localState.setSshStage("picker");
      toast.message("Setup cancelled.");
    },
  };
}

/**
 * Add computer modal (`_uiux.md` U2):
 * - Three mutually exclusive paths via `RadioCardGroup` (SSH, On your account, Link with a command)
 * - Deterministic 7-step SSH progress list with inline anchored Trust / Password prompts
 * - Safe Cancel confirmation before `Linking` commits; honest `Remove` afterwards (Rule 19)
 */
export function AddComputerDialog({
  open,
  accountEmail,
  isFreePlan = false,
  sshSuggestions,
  unaddedComputers,
  initialMethod = "ssh",
  initialSshStage = "picker",
  initialHostDraft = DEFAULT_DRAFT,
  defaultHostPickerOpen = false,
  initialCommandDetected = false,
  linkedCommitReached = false,
  live,
  onClose,
  onUpgrade,
  onOpenComputer,
}: AddComputerDialogProps) {
  const localState = useAddComputerDialog({
    isFreePlan,
    sshSuggestions,
    initialMethod,
    initialSshStage,
    initialHostDraft,
    linkedCommitReached,
    onClose,
  });
  const [liveCancelConfirmOpen, setLiveCancelConfirmOpen] = useState(false);
  const view = live
    ? buildLiveDialogView(
        live,
        sshSuggestions,
        liveCancelConfirmOpen,
        setLiveCancelConfirmOpen,
        localState.setSshStage,
        onClose
      )
    : buildLocalDialogView(localState, initialCommandDetected, linkedCommitReached);
  const methodOptions = buildMethodOptions(unaddedComputers.length);

  return (
    <>
      <Dialog
        onOpenChange={next => {
          if (!next) view.handleRequestCancel();
        }}
        open={open}
      >
        <DialogContent
          className={cn(dialogShellClass("md"), "flex flex-col")}
          showCloseButton={false}
          unframed
        >
          <EntityDialogHeader
            description="Connect another machine so your sessions, terminals, and files run there."
            eyebrow="Computers"
            icon={Laptop}
            onClose={view.handleRequestCancel}
            title="Add computer"
          />

          <EntityDialogBody className="flex flex-col gap-5">
            {!view.isSshProgressView ? (
              <RadioCardGroup
                aria-label="How to add a computer"
                className={methodOptions.length === 3 ? "sm:grid-cols-3" : "sm:grid-cols-2"}
                iconWellSize="lg"
                onValueChange={view.selectMethod}
                options={methodOptions}
                value={view.method}
              />
            ) : null}

            {view.method === "ssh" && view.sshStage === "picker" ? (
              <AddComputerSshPicker
                defaultPickerOpen={defaultHostPickerOpen}
                onChange={view.setDraft}
                onSwitchToCommand={view.switchToCommand}
                suggestions={sshSuggestions}
                value={view.draft}
              />
            ) : null}

            {view.isSshProgressView ? (
              <AddComputerSshProgressView
                accountEmail={accountEmail}
                hostLabel={view.hostLabel}
                live={live}
                onRequestCancel={view.handleRequestCancel}
                onSetSshStage={view.setSshStage}
                onSwitchToCommand={view.switchToCommand}
                sshStage={view.sshStage}
                targetAddress={view.targetAddress}
              />
            ) : null}

            {view.method === "account" ? (
              <AddComputerAccountPath
                accountEmail={accountEmail}
                addedIds={view.addedIds}
                isFreePlan={isFreePlan}
                onAdd={view.onAddAccountComputer}
                onUpgrade={onUpgrade}
                unaddedComputers={unaddedComputers}
              />
            ) : null}

            {view.method === "command" ? (
              <AddComputerCommandPath
                accountEmail={accountEmail}
                addedIds={view.addedIds}
                detected={view.commandDetected}
                detectedComputer={live?.detectedCommandComputer}
                onAdd={view.onAddAccountComputer}
              />
            ) : null}
          </EntityDialogBody>

          <AddComputerFooter
            accountEmail={accountEmail}
            commandDetected={view.commandDetected}
            hostLabel={view.hostLabel}
            linkedCommitReached={view.resolvedLinkedCommit}
            method={view.method}
            onClose={() => onClose?.()}
            onOpenComputer={onOpenComputer}
            onRemoveLinked={view.onRemoveLinked}
            onRequestCancel={view.handleRequestCancel}
            onRetry={view.onRetry}
            onStartSsh={view.onStartSsh}
            onSwitchToCommand={view.switchToCommand}
            onUpgrade={onUpgrade}
            pickerBlocked={view.pickerBlocked}
            sshStage={view.sshStage}
          />
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        cancelLabel="Keep adding"
        confirmLabel="Stop setup"
        description={`Stops adding ${view.hostLabel} right now and frees your computer slot. Nothing further will be changed on ${view.targetAddress}.`}
        onConfirm={view.handleConfirmCancel}
        onOpenChange={view.setCancelConfirmOpen}
        open={view.cancelConfirmOpen}
        title={`Stop adding ${view.hostLabel}?`}
        tone="warning"
      />
    </>
  );
}
