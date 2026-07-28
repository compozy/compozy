export interface SettingsRestartViewState {
  isVisible: boolean;
  isRestartRequired: boolean;
  isPolling: boolean;
  isSuccessful: boolean;
  isFailed: boolean;
  operationId: string | null;
  status: string | null;
  failureReason?: string;
  activeSessionCount: number;
  isTriggerPending: boolean;
  trigger: () => void;
  dismiss: () => void;
}

export type SettingsRestartPhase = "required" | "polling" | "successful" | "failed";
export type SettingsRestartTone = "warning" | "info" | "success" | "danger";

export interface SettingsRestartPresentation {
  phase: SettingsRestartPhase;
  tone: SettingsRestartTone;
  title: string;
  body: string;
  role: "status" | "alert";
  sessionLabel: string | null;
  dismissLabel: string | null;
  triggerLabel: string | null;
  triggerPending: boolean;
}

function sessionLabelFor(state: SettingsRestartViewState): string | null {
  if (!state.isRestartRequired || state.isPolling || state.activeSessionCount <= 0) {
    return null;
  }
  const noun = state.activeSessionCount === 1 ? "session" : "sessions";
  return `${state.activeSessionCount} active ${noun} right now.`;
}

export function settingsRestartPresentation(
  state: SettingsRestartViewState
): SettingsRestartPresentation | null {
  if (!state.isVisible) return null;

  const sessionLabel = sessionLabelFor(state);
  if (state.isFailed) {
    return {
      phase: "failed",
      tone: "danger",
      title: "Restart failed",
      body:
        state.failureReason?.trim() ||
        "The daemon did not come back. Check its logs, then try again.",
      role: "alert",
      sessionLabel,
      dismissLabel: "Dismiss",
      triggerLabel: state.isTriggerPending ? "Retrying…" : "Try again",
      triggerPending: state.isTriggerPending,
    };
  }
  if (state.isSuccessful) {
    return {
      phase: "successful",
      tone: "success",
      title: "Restarted",
      body: "All saved changes are now active.",
      role: "status",
      sessionLabel: null,
      dismissLabel: "Dismiss",
      triggerLabel: null,
      triggerPending: false,
    };
  }
  if (state.isPolling) {
    return {
      phase: "polling",
      tone: "info",
      title: "Restarting…",
      body: "The daemon comes back in a few seconds. Sessions reconnect on their own.",
      role: "status",
      sessionLabel: null,
      dismissLabel: null,
      triggerLabel: null,
      triggerPending: false,
    };
  }
  return {
    phase: "required",
    tone: "warning",
    title: "Restart needed",
    body: "Some saved changes apply after the daemon restarts. Running sessions keep their current rules until then.",
    role: "status",
    sessionLabel,
    dismissLabel: state.isTriggerPending ? null : "Not now",
    triggerLabel: state.isTriggerPending ? "Starting…" : "Restart daemon",
    triggerPending: state.isTriggerPending,
  };
}
