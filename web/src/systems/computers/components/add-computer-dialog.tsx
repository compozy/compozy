import {
  CheckCircle2,
  Laptop,
  Link2,
  Sparkles,
  Terminal,
  TriangleAlert,
  UserCheck,
} from "@compozy/ui/icons";

import {
  ActionResultBanner,
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  cn,
  CodeBlock,
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
import type {
  AccountUnaddedComputerFixture,
  SshHostSuggestionFixture,
} from "../mocks/remote-computers-fixtures";
import {
  buildSshSetupSteps,
  SSH_HOST_KEY_CHANGED_FIXTURE,
  SSH_HOST_TRUST_FIXTURE,
  SSH_LINGER_NOTE_FIXTURE,
  SSH_SECRET_PROMPT_FIXTURE,
  type AddComputerMethod,
  type AddComputerSshStage,
} from "../mocks/remote-computers-fixtures-lane-b";
import {
  AddComputerAccountPath,
  AddComputerCommandPath,
  AddComputerFooter,
} from "./add-computer-paths";
import { AddComputerSshPicker, type SshTargetDraft } from "./add-computer-ssh-picker";
import { AddComputerHostKeyCard, AddComputerSecretCard } from "./add-computer-ssh-prompt";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { StepProgressList } from "./step-progress-list";

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

function AddComputerSshProgressView({
  sshStage,
  hostLabel,
  targetAddress,
  onSetSshStage,
  onSwitchToCommand,
  onRequestCancel,
}: {
  sshStage: AddComputerSshStage;
  hostLabel: string;
  targetAddress: string;
  onSetSshStage: (stage: AddComputerSshStage) => void;
  onSwitchToCommand: () => void;
  onRequestCancel: () => void;
}) {
  const steps = buildSshSetupSteps({
    stage: sshStage,
    hostLabel,
    targetAddress,
    trustPromptNode:
      sshStage === "host_key_changed" ? (
        <AddComputerHostKeyCard
          fingerprint={SSH_HOST_KEY_CHANGED_FIXTURE.observedFingerprint}
          hostLabel={hostLabel}
          keyType={SSH_HOST_KEY_CHANGED_FIXTURE.keyType}
          mode="changed"
          pinEntry={SSH_HOST_KEY_CHANGED_FIXTURE.pinEntry}
          previousFingerprint={SSH_HOST_KEY_CHANGED_FIXTURE.pinnedFingerprint}
          recoveryCommand={SSH_HOST_KEY_CHANGED_FIXTURE.recoveryCommand}
          target={SSH_HOST_KEY_CHANGED_FIXTURE.target}
          verifyCommand={SSH_HOST_KEY_CHANGED_FIXTURE.verifyCommand}
        />
      ) : sshStage === "trust_host" ? (
        <AddComputerHostKeyCard
          fingerprint={SSH_HOST_TRUST_FIXTURE.fingerprint}
          hostLabel={hostLabel}
          keyType={SSH_HOST_TRUST_FIXTURE.keyType}
          mode="trust"
          onDecline={() => {
            onSetSshStage("picker");
            toast.message("Setup cancelled. Host key was not trusted.");
          }}
          onTrust={() => {
            onSetSshStage("running");
            toast.success(`Trusted ${hostLabel} and continued setup.`);
          }}
          target={SSH_HOST_TRUST_FIXTURE.target}
          verifyCommand={SSH_HOST_TRUST_FIXTURE.verifyCommand}
        />
      ) : undefined,
    secretPromptNode:
      sshStage === "password" || sshStage === "agent_secret" ? (
        <AddComputerSecretCard
          agentSessionTitle={
            sshStage === "agent_secret" ? SSH_SECRET_PROMPT_FIXTURE.agentSessionTitle : null
          }
          countdownSeconds={SSH_SECRET_PROMPT_FIXTURE.countdownSeconds}
          onCancel={onRequestCancel}
          onRetry={() => onSetSshStage("running")}
          onSubmit={() => {
            onSetSshStage("running");
            toast.success("Authenticated with SSH.");
          }}
          promptLabel={SSH_SECRET_PROMPT_FIXTURE.promptLabel}
          target={targetAddress}
        />
      ) : undefined,
    planLimitPromptNode:
      sshStage === "plan_limit" ? (
        <Alert variant="warning">
          <Sparkles aria-hidden="true" />
          <AlertTitle>Free includes 1 remote computer</AlertTitle>
          <AlertDescription>
            Upgrade to Pro for unlimited computers. Nothing was installed or changed on {hostLabel}.
          </AlertDescription>
        </Alert>
      ) : undefined,
    unsupportedPromptNode:
      sshStage === "unsupported" ? (
        <Alert variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>Link this computer with a command instead</AlertTitle>
          <AlertDescription className="flex flex-col gap-2">
            <span>
              Windows, 32-bit, and FreeBSD computers connect through{" "}
              <span className="font-mono text-mono-id text-fg">compozy remote link</span>.
            </span>
            <Button
              className="w-fit"
              onClick={onSwitchToCommand}
              size="xs"
              type="button"
              variant="secondary"
            >
              Show link command
            </Button>
          </AlertDescription>
        </Alert>
      ) : undefined,
  });

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-2 border-b border-line-soft pb-3">
        <div className="flex min-w-0 items-center gap-2.5">
          <ComputerPlatformIcon platform="linux_x86_64" size="sm" tone="muted" />
          <div className="min-w-0">
            <p className="truncate text-small-body font-medium text-fg-strong">{hostLabel}</p>
            <p className="truncate font-mono text-mono-id text-muted-foreground">{targetAddress}</p>
          </div>
        </div>
        {sshStage === "failed" || sshStage === "host_key_changed" || sshStage === "unsupported" ? (
          <Button onClick={() => onSetSshStage("picker")} size="xs" type="button" variant="ghost">
            Change host
          </Button>
        ) : null}
      </div>

      <StepProgressList data-testid="add-computer-ssh-steps" steps={steps} />

      {sshStage === "done" ? (
        <div className="flex flex-col gap-3">
          <ActionResultBanner
            description={`Connected via SSH (${targetAddress}). Sessions you start on ${hostLabel} keep running even if you close this app.`}
            icon={CheckCircle2}
            title={`${hostLabel} is ready`}
            tone="success"
          />
          <Alert variant="info">
            <TriangleAlert aria-hidden="true" />
            <AlertTitle>{SSH_LINGER_NOTE_FIXTURE.summary}</AlertTitle>
            <AlertDescription className="flex flex-col gap-2">
              <span>{SSH_LINGER_NOTE_FIXTURE.detail}</span>
              <CodeBlock
                code={SSH_LINGER_NOTE_FIXTURE.command}
                copyable
                density="compact"
                language="bash"
                showPrompt
              />
            </AlertDescription>
          </Alert>
        </div>
      ) : null}
    </div>
  );
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
  onClose,
  onUpgrade,
  onOpenComputer,
}: AddComputerDialogProps) {
  const state = useAddComputerDialog({
    isFreePlan,
    sshSuggestions,
    initialMethod,
    initialSshStage,
    initialHostDraft,
    linkedCommitReached,
    onClose,
  });

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
          if (!next) state.handleRequestCancel();
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
            onClose={state.handleRequestCancel}
            title="Add computer"
          />

          <EntityDialogBody className="flex flex-col gap-5">
            {!state.isSshProgressView ? (
              <RadioCardGroup
                aria-label="How to add a computer"
                className={methodOptions.length === 3 ? "sm:grid-cols-3" : "sm:grid-cols-2"}
                iconWellSize="lg"
                onValueChange={next => {
                  state.setMethod(next);
                  state.setSshStage("picker");
                }}
                options={methodOptions}
                value={state.method}
              />
            ) : null}

            {state.method === "ssh" && state.sshStage === "picker" ? (
              <AddComputerSshPicker
                defaultPickerOpen={defaultHostPickerOpen}
                onChange={state.setDraft}
                onSwitchToCommand={() => state.setMethod("command")}
                suggestions={sshSuggestions}
                value={state.draft}
              />
            ) : null}

            {state.isSshProgressView ? (
              <AddComputerSshProgressView
                hostLabel={state.hostLabel}
                onRequestCancel={state.handleRequestCancel}
                onSetSshStage={state.setSshStage}
                onSwitchToCommand={() => {
                  state.setMethod("command");
                  state.setSshStage("picker");
                }}
                sshStage={state.sshStage}
                targetAddress={state.targetAddress}
              />
            ) : null}

            {state.method === "account" ? (
              <AddComputerAccountPath
                accountEmail={accountEmail}
                addedIds={state.addedIds}
                isFreePlan={isFreePlan}
                onAdd={state.handleAddAccountItem}
                onUpgrade={onUpgrade}
                unaddedComputers={unaddedComputers}
              />
            ) : null}

            {state.method === "command" ? (
              <AddComputerCommandPath
                accountEmail={accountEmail}
                addedIds={state.addedIds}
                detected={initialCommandDetected}
                onAdd={state.handleAddAccountItem}
              />
            ) : null}
          </EntityDialogBody>

          <AddComputerFooter
            accountEmail={accountEmail}
            commandDetected={initialCommandDetected}
            hostLabel={state.hostLabel}
            linkedCommitReached={linkedCommitReached}
            method={state.method}
            onClose={() => onClose?.()}
            onOpenComputer={onOpenComputer}
            onRemoveLinked={() => toast.message(`Opens Remove dialog for ${state.hostLabel}`)}
            onRequestCancel={state.handleRequestCancel}
            onRetry={state.handleRetryFailed}
            onStartSsh={state.handleStartSsh}
            onSwitchToCommand={() => {
              state.setMethod("command");
              state.setSshStage("picker");
            }}
            onUpgrade={onUpgrade}
            pickerBlocked={state.pickerBlocked}
            sshStage={state.sshStage}
          />
        </DialogContent>
      </Dialog>

      <ConfirmDialog
        cancelLabel="Keep adding"
        confirmLabel="Stop setup"
        description={`Stops adding ${state.hostLabel} right now and frees your computer slot. Nothing further will be changed on ${state.targetAddress}.`}
        onConfirm={() => {
          state.setCancelConfirmOpen(false);
          state.setSshStage("picker");
          toast.message("Setup cancelled.");
        }}
        onOpenChange={state.setCancelConfirmOpen}
        open={state.cancelConfirmOpen}
        title={`Stop adding ${state.hostLabel}?`}
        tone="warning"
      />
    </>
  );
}
