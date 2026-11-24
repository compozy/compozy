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

  const method = live ? live.method : localState.method;
  const draft = live ? live.draft : localState.draft;
  const sshStage = live ? live.sshStage : localState.sshStage;
  const isSshProgressView = method === "ssh" && sshStage !== "picker";
  const resolvedLinkedCommit = live ? live.linkedCommitReached : linkedCommitReached;
  const addedIds = live ? live.addedIds : localState.addedIds;
  const commandDetected = live ? Boolean(live.detectedCommandComputer) : initialCommandDetected;

  const { targetAddress, hostLabel, pickerBlocked } = live
    ? resolveTarget(draft, sshSuggestions)
    : localState;

  const cancelConfirmOpen = live ? liveCancelConfirmOpen : localState.cancelConfirmOpen;
  const setCancelConfirmOpen = live ? setLiveCancelConfirmOpen : localState.setCancelConfirmOpen;

  const handleRequestCancel = () => {
    if (live) {
      if (isSshProgressView && ACTIVE_SSH_STAGES.has(sshStage) && !resolvedLinkedCommit) {
        setLiveCancelConfirmOpen(true);
        return;
      }
      onClose?.();
      return;
    }
    localState.handleRequestCancel();
  };

  const accountCountLabel =
    unaddedComputers.length === 1
      ? "1 computer ready to add"
      : `${unaddedComputers.length} computers ready to add`;

  const methodOptions: Array<RadioCardGroupOption<AddComputerMethod>> = [
    {
      value: "ssh",
      title: "SSH",
      description: "Install and connect over SSH using your SSH config",
      icon: Terminal,
    },
    ...(unaddedComputers.length > 0
      ? [
          {
            value: "account" as const,
            title: "On your account",
            description: accountCountLabel,
            icon: UserCheck,
          },
        ]
      : []),
    {
      value: "command",
      title: "Link with a command",
      description: "Run one command on the other computer, including Windows",
      icon: Link2,
    },
  ];

  return (
    <>
      <Dialog
        onOpenChange={next => {
          if (!next) handleRequestCancel();
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
            onClose={handleRequestCancel}
            title="Add computer"
          />

          <EntityDialogBody className="flex flex-col gap-5">
            {!isSshProgressView ? (
              <RadioCardGroup
                aria-label="How to add a computer"
                className={methodOptions.length === 3 ? "sm:grid-cols-3" : "sm:grid-cols-2"}
                iconWellSize="lg"
                onValueChange={next => {
                  if (live) {
                    live.setMethod(next);
                    live.onBackToPicker();
                  } else {
                    localState.setMethod(next);
                    localState.setSshStage("picker");
                  }
                }}
                options={methodOptions}
                value={method}
              />
            ) : null}

            {method === "ssh" && sshStage === "picker" ? (
              <AddComputerSshPicker
                defaultPickerOpen={defaultHostPickerOpen}
                onChange={live ? live.setDraft : localState.setDraft}
                onSwitchToCommand={() =>
                  live ? live.setMethod("command") : localState.setMethod("command")
                }
                suggestions={sshSuggestions}
                value={draft}
              />
            ) : null}

            {isSshProgressView ? (
              <AddComputerSshProgressView
                accountEmail={accountEmail}
                hostLabel={hostLabel}
                live={live}
                onRequestCancel={handleRequestCancel}
                onSetSshStage={localState.setSshStage}
                onSwitchToCommand={() => {
                  if (live) {
                    live.setMethod("command");
                    live.onBackToPicker();
                  } else {
                    localState.setMethod("command");
                    localState.setSshStage("picker");
                  }
                }}
                sshStage={sshStage}
                targetAddress={targetAddress}
              />
            ) : null}

            {method === "account" ? (
              <AddComputerAccountPath
                accountEmail={accountEmail}
                addedIds={addedIds}
                isFreePlan={isFreePlan}
                onAdd={live ? live.onAddAccountComputer : localState.handleAddAccountItem}
                onUpgrade={onUpgrade}
                unaddedComputers={unaddedComputers}
              />
            ) : null}

            {method === "command" ? (
              <AddComputerCommandPath
                accountEmail={accountEmail}
                addedIds={addedIds}
                detected={commandDetected}
                detectedComputer={live?.detectedCommandComputer}
                onAdd={live ? live.onAddAccountComputer : localState.handleAddAccountItem}
              />
            ) : null}
          </EntityDialogBody>

          <AddComputerFooter
            accountEmail={accountEmail}
            commandDetected={commandDetected}
            hostLabel={hostLabel}
            linkedCommitReached={resolvedLinkedCommit}
            method={method}
            onClose={() => onClose?.()}
            onOpenComputer={onOpenComputer}
            onRemoveLinked={
              live
                ? live.onRemoveLinked
                : () => toast.message(`Opens Remove dialog for ${hostLabel}`)
            }
            onRequestCancel={handleRequestCancel}
            onRetry={live ? live.onRetrySetup : localState.handleRetryFailed}
            onStartSsh={live ? live.onStartSsh : localState.handleStartSsh}
            onSwitchToCommand={() => {
              if (live) {
                live.setMethod("command");
                live.onBackToPicker();
              } else {
                localState.setMethod("command");
                localState.setSshStage("picker");
              }
            }}
            onUpgrade={onUpgrade}
            pickerBlocked={pickerBlocked}
            sshStage={sshStage}
          />
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        cancelLabel="Keep adding"
        confirmLabel="Stop setup"
        description={`Stops adding ${hostLabel} right now and frees your computer slot. Nothing further will be changed on ${targetAddress}.`}
        onConfirm={() => {
          setCancelConfirmOpen(false);
          if (live) {
            live.onConfirmCancel();
          } else {
            localState.setSshStage("picker");
            toast.message("Setup cancelled.");
          }
        }}
        onOpenChange={setCancelConfirmOpen}
        open={cancelConfirmOpen}
        title={`Stop adding ${hostLabel}?`}
        tone="warning"
      />
    </>
  );
}
