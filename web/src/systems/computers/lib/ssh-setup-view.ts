import type { StepProgressItem } from "../components/step-progress-list";
import type {
  AddComputerSshStage,
  UpdateRunState,
} from "../mocks/remote-computers-fixtures-lane-b";
import type { ComputerSetupStatusPayload } from "../types";

export const SSH_SETUP_STEPS = [
  { id: "admit", title: "Checking your plan" },
  { id: "verify", title: "Verifying release" },
  { id: "connect", title: "Connecting over SSH" },
  { id: "install", title: "Installing CompozyOS" },
  { id: "service", title: "Starting CompozyOS service" },
  { id: "link", title: "Linking to your account" },
  { id: "forward", title: "Opening the connection" },
] as const;

export const SSH_UPDATE_STEPS = [
  { id: "verify", title: "Verifying release" },
  { id: "install", title: "Installing CompozyOS" },
  { id: "service", title: "Restarting service" },
  { id: "forward", title: "Opening the connection" },
] as const;

export interface LiveHostKeyPromptInfo {
  mode: "trust" | "changed";
  target: string;
  pinEntry: string;
  keyType: string;
  fingerprint: string;
  previousFingerprint?: string;
  recoveryCommand?: string;
}

export interface LiveSecretPromptInfo {
  target: string;
  promptLabel: string;
  countdownSeconds: number;
  agentSessionTitle?: string | null;
}

export function formatKnownHostsPinEntry(host: string, port?: string | number): string {
  const cleanHost = host.trim();
  const parsedPort = typeof port === "number" ? port : Number.parseInt(port ?? "22", 10);
  if (!Number.isFinite(parsedPort) || parsedPort === 22) {
    return cleanHost;
  }
  return `[${cleanHost}]:${parsedPort}`;
}

export function buildHostKeyRecoveryCommand(pinEntry: string): string {
  return `ssh-keygen -R '${pinEntry}' -f ~/.compozy/ssh/known_hosts`;
}

export function isSetupLinkedCommitReached(status: ComputerSetupStatusPayload | null): boolean {
  if (!status) return false;
  if (status.computer_id) return true;
  if (status.step === "forward") return true;
  if (status.state === "done") return true;
  return false;
}

/** Maps a live `ComputerSetupStatusPayload` into the UI stage for `AddComputerDialog`. */
export function toLiveSshStage(
  status: ComputerSetupStatusPayload | null,
  options: { isAgentInitiated?: boolean } = {}
): AddComputerSshStage {
  if (!status) return "picker";
  if (status.state === "done") return "done";
  if (status.state === "canceled") return "picker";

  const errorCode = status.error?.code;
  if (errorCode === "ssh_host_key_changed") return "host_key_changed";
  if (errorCode === "plan_limit_reached") return "plan_limit";
  if (errorCode === "ssh_unsupported_platform") return "unsupported";

  const promptKind = status.prompt?.kind ?? status.prompt_kind;
  if (status.state === "needs_host_key" || promptKind === "host_key") {
    return "trust_host";
  }
  if (status.state === "needs_operator_secret" || promptKind === "secret") {
    return options.isAgentInitiated ? "agent_secret" : "password";
  }

  if (status.state === "failed") return "failed";
  return "running";
}

export function extractLiveHostKeyPrompt(
  status: ComputerSetupStatusPayload | null,
  fallbackHost: string,
  fallbackPort?: string
): LiveHostKeyPromptInfo | null {
  if (!status) return null;
  const errorCode = status.error?.code;
  const details = status.error?.details ?? {};

  if (errorCode === "ssh_host_key_changed") {
    const host = details.host || status.prompt?.host || fallbackHost;
    const pinEntry = details.pin_entry || formatKnownHostsPinEntry(host, fallbackPort);
    return {
      mode: "changed",
      target: host,
      pinEntry,
      keyType: details.key_type || "ED25519",
      fingerprint:
        details.observed_fingerprint ||
        details.fingerprint ||
        status.prompt?.fingerprint ||
        "SHA256:unknown",
      previousFingerprint: details.pinned_fingerprint || undefined,
      recoveryCommand: details.recovery_command || buildHostKeyRecoveryCommand(pinEntry),
    };
  }

  const promptKind = status.prompt?.kind ?? status.prompt_kind;
  if (status.state === "needs_host_key" || promptKind === "host_key") {
    const host = status.prompt?.host || fallbackHost;
    const pinEntry = formatKnownHostsPinEntry(host, fallbackPort);
    return {
      mode: "trust",
      target: host,
      pinEntry,
      keyType: "ED25519",
      fingerprint: status.prompt?.fingerprint || "SHA256:pending",
    };
  }

  return null;
}

export interface BuildLiveSshStepsInput {
  status: ComputerSetupStatusPayload;
  hostLabel: string;
  targetAddress: string;
  accountEmail: string;
  trustPromptNode?: StepProgressItem["prompt"];
  secretPromptNode?: StepProgressItem["prompt"];
  planLimitPromptNode?: StepProgressItem["prompt"];
  unsupportedPromptNode?: StepProgressItem["prompt"];
}

/** Projects a live `ComputerSetupStatusPayload` into the 7-step `StepProgressItem[]` list. */
export function buildLiveSshSetupSteps({
  status,
  hostLabel,
  targetAddress,
  accountEmail,
  trustPromptNode,
  secretPromptNode,
  planLimitPromptNode,
  unsupportedPromptNode,
}: BuildLiveSshStepsInput): StepProgressItem[] {
  const stage = toLiveSshStage(status);
  const currentStepId = status.step || "admit";
  const currentIdx = Math.max(
    0,
    SSH_SETUP_STEPS.findIndex(item => item.id === currentStepId)
  );
  const errorDetails = status.error?.details ?? {};
  const outputLine = errorDetails.last_line || errorDetails.output || undefined;

  return SSH_SETUP_STEPS.map((step, idx) => {
    if (stage === "done") {
      return {
        id: step.id,
        title: step.title,
        status: "done",
        detail:
          step.id === "link"
            ? accountEmail
            : step.id === "forward"
              ? `via SSH ${targetAddress}`
              : undefined,
      };
    }

    if (idx < currentIdx) {
      return {
        id: step.id,
        title: step.title,
        status: "done",
      };
    }

    if (idx > currentIdx) {
      return {
        id: step.id,
        title: step.title,
        status: "pending",
      };
    }

    // Current step
    if (stage === "trust_host") {
      return {
        id: step.id,
        title: step.title,
        status: "needs_input",
        detail: targetAddress,
        caption: "Verify this computer's host key before continuing.",
        prompt: trustPromptNode,
      };
    }

    if (stage === "host_key_changed") {
      return {
        id: step.id,
        title: step.title,
        status: "failed",
        detail: targetAddress,
        prompt: trustPromptNode,
      };
    }

    if (stage === "password" || stage === "agent_secret") {
      return {
        id: step.id,
        title: step.title,
        status: "needs_input",
        detail: targetAddress,
        caption: `${hostLabel} asked for a password or one-time code.`,
        prompt: secretPromptNode,
      };
    }

    if (stage === "plan_limit") {
      return {
        id: step.id,
        title: step.title,
        status: "failed",
        caption: status.error?.error || "Free includes 1 remote computer.",
        prompt: planLimitPromptNode,
      };
    }

    if (stage === "unsupported") {
      return {
        id: step.id,
        title: step.title,
        status: "failed",
        caption:
          status.error?.error ||
          "This computer's system isn't supported over SSH. Link it with compozy remote link instead.",
        prompt: unsupportedPromptNode,
      };
    }

    if (stage === "failed") {
      return {
        id: step.id,
        title: step.title,
        status: "failed",
        caption: status.error?.error || `Step "${step.title}" failed on ${hostLabel}.`,
        outputLine,
      };
    }

    return {
      id: step.id,
      title: step.title,
      status: "running",
      detail: step.id === "connect" ? targetAddress : undefined,
    };
  });
}

export function toLiveUpdateState(status: ComputerSetupStatusPayload | null): UpdateRunState {
  if (!status) return "idle";
  if (status.state === "done") return "done";
  if (status.state === "failed") {
    return "provenance_failed";
  }
  return "running";
}

export function buildLiveUpdateSteps(
  status: ComputerSetupStatusPayload | null,
  computerName: string
): StepProgressItem[] {
  if (!status) {
    return SSH_UPDATE_STEPS.map(step => ({
      id: step.id,
      title: step.id === "service" ? `Restarting service on ${computerName}` : step.title,
      status: "pending",
    }));
  }

  const currentStepId = status.step || "verify";
  const currentIdx = Math.max(
    0,
    SSH_UPDATE_STEPS.findIndex(item => item.id === currentStepId)
  );
  const outputLine = status.error?.details?.last_line || undefined;

  return SSH_UPDATE_STEPS.map((step, idx) => {
    const title = step.id === "service" ? `Restarting service on ${computerName}` : step.title;
    if (status.state === "done") {
      return { id: step.id, title, status: "done" };
    }
    if (idx < currentIdx) {
      return { id: step.id, title, status: "done" };
    }
    if (idx > currentIdx) {
      return { id: step.id, title, status: "pending" };
    }
    if (status.state === "failed") {
      return {
        id: step.id,
        title,
        status: "failed",
        caption: status.error?.error || "Update failed.",
        outputLine,
      };
    }
    return { id: step.id, title, status: "running" };
  });
}
