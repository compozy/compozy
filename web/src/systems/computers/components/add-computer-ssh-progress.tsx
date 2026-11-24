import type { ReactNode } from "react";
import { CheckCircle2, Sparkles, TriangleAlert } from "@compozy/ui/icons";

import {
  ActionResultBanner,
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  CodeBlock,
  toast,
} from "@compozy/ui";

import type { AddComputerLiveController } from "../hooks/use-live-add-computer";
import { buildLiveSshSetupSteps } from "../lib/ssh-setup-view";
import {
  buildSshSetupSteps,
  SSH_HOST_KEY_CHANGED_FIXTURE,
  SSH_HOST_TRUST_FIXTURE,
  SSH_LINGER_NOTE_FIXTURE,
  SSH_SECRET_PROMPT_FIXTURE,
  type AddComputerSshStage,
} from "../mocks/remote-computers-fixtures-lane-b";
import { AddComputerHostKeyCard, AddComputerSecretCard } from "./add-computer-ssh-prompt";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { StepProgressList } from "./step-progress-list";

export interface AddComputerSshProgressViewProps {
  sshStage: AddComputerSshStage;
  hostLabel: string;
  targetAddress: string;
  accountEmail: string;
  live?: AddComputerLiveController;
  onSetSshStage: (stage: AddComputerSshStage) => void;
  onSwitchToCommand: () => void;
  onRequestCancel: () => void;
}

function renderTrustPrompt({
  sshStage,
  hostLabel,
  live,
  onSetSshStage,
}: Pick<AddComputerSshProgressViewProps, "sshStage" | "hostLabel" | "live" | "onSetSshStage">):
  | ReactNode
  | undefined {
  if (live?.hostKeyPrompt) {
    const prompt = live.hostKeyPrompt;
    return (
      <AddComputerHostKeyCard
        fingerprint={prompt.fingerprint}
        hostLabel={hostLabel}
        keyType={prompt.keyType}
        mode={prompt.mode}
        onDecline={prompt.mode === "trust" ? live.onDeclineHost : undefined}
        onTrust={prompt.mode === "trust" ? live.onTrustHost : undefined}
        pinEntry={prompt.pinEntry}
        previousFingerprint={prompt.previousFingerprint}
        recoveryCommand={prompt.recoveryCommand}
        target={prompt.target}
      />
    );
  }
  if (sshStage === "host_key_changed") {
    return (
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
    );
  }
  if (sshStage === "trust_host") {
    return (
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
    );
  }
  return undefined;
}

function renderSecretPrompt({
  sshStage,
  targetAddress,
  live,
  onSetSshStage,
  onRequestCancel,
}: Pick<
  AddComputerSshProgressViewProps,
  "sshStage" | "targetAddress" | "live" | "onSetSshStage" | "onRequestCancel"
>): ReactNode | undefined {
  if (sshStage !== "password" && sshStage !== "agent_secret") return undefined;
  const agentSessionTitle = live
    ? live.isAgentInitiated
      ? "Agent setup"
      : null
    : sshStage === "agent_secret"
      ? SSH_SECRET_PROMPT_FIXTURE.agentSessionTitle
      : null;

  return (
    <AddComputerSecretCard
      agentSessionTitle={agentSessionTitle}
      countdownSeconds={
        live ? live.secretCountdownSeconds : SSH_SECRET_PROMPT_FIXTURE.countdownSeconds
      }
      errorMessage={live?.secretError}
      onCancel={onRequestCancel}
      onRetry={live ? live.onRetrySetup : () => onSetSshStage("running")}
      onSubmit={
        live
          ? live.onSubmitSecret
          : () => {
              onSetSshStage("running");
              toast.success("Authenticated with SSH.");
            }
      }
      promptLabel={SSH_SECRET_PROMPT_FIXTURE.promptLabel}
      target={targetAddress}
    />
  );
}

function SshDoneSummary({
  hostLabel,
  targetAddress,
  showDraftLingerNote,
}: {
  hostLabel: string;
  targetAddress: string;
  showDraftLingerNote: boolean;
}) {
  return (
    <div className="flex flex-col gap-3">
      <ActionResultBanner
        description={`Connected via SSH (${targetAddress}). Sessions you start on ${hostLabel} keep running even if you close this app.`}
        icon={CheckCircle2}
        title={`${hostLabel} is ready`}
        tone="success"
      />
      {showDraftLingerNote ? (
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
      ) : null}
    </div>
  );
}

const CHANGE_HOST_STAGES = new Set<AddComputerSshStage>([
  "failed",
  "host_key_changed",
  "unsupported",
]);

export function AddComputerSshProgressView({
  sshStage,
  hostLabel,
  targetAddress,
  accountEmail,
  live,
  onSetSshStage,
  onSwitchToCommand,
  onRequestCancel,
}: AddComputerSshProgressViewProps) {
  const trustPromptNode = renderTrustPrompt({ sshStage, hostLabel, live, onSetSshStage });
  const secretPromptNode = renderSecretPrompt({
    sshStage,
    targetAddress,
    live,
    onSetSshStage,
    onRequestCancel,
  });
  const planLimitPromptNode =
    sshStage === "plan_limit" ? (
      <Alert variant="warning">
        <Sparkles aria-hidden="true" />
        <AlertTitle>Free includes 1 remote computer</AlertTitle>
        <AlertDescription>
          Upgrade to Pro for unlimited computers. Nothing was installed or changed on {hostLabel}.
        </AlertDescription>
      </Alert>
    ) : undefined;
  const unsupportedPromptNode =
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
    ) : undefined;

  const steps =
    live?.setupStatus != null
      ? buildLiveSshSetupSteps({
          status: live.setupStatus,
          hostLabel,
          targetAddress,
          accountEmail,
          trustPromptNode,
          secretPromptNode,
          planLimitPromptNode,
          unsupportedPromptNode,
        })
      : buildSshSetupSteps({
          stage: sshStage,
          hostLabel,
          targetAddress,
          trustPromptNode,
          secretPromptNode,
          planLimitPromptNode,
          unsupportedPromptNode,
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
        {CHANGE_HOST_STAGES.has(sshStage) ? (
          <Button
            onClick={() => (live ? live.onBackToPicker() : onSetSshStage("picker"))}
            size="xs"
            type="button"
            variant="ghost"
          >
            Change host
          </Button>
        ) : null}
      </div>

      <StepProgressList data-testid="add-computer-ssh-steps" steps={steps} />

      {sshStage === "done" ? (
        <SshDoneSummary
          hostLabel={hostLabel}
          showDraftLingerNote={!live}
          targetAddress={targetAddress}
        />
      ) : null}
    </div>
  );
}
