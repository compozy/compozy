import { WindowManagerApiError } from "../adapters/window-manager-api";
import type {
  WindowManagerCommandInput,
  WindowManagerCommandResult,
  WindowManagerDiagnosticPayload,
} from "../lib/window-manager-types";
import { windowManagerStore, type WindowManagerBinding } from "../stores/window-manager-store";

/**
 * How a dispatched command's result or failure reaches the interaction store.
 *
 * Kept apart from the runtime that dispatches: this half owns only the vocabulary
 * the surface reads — completed, failed, conflicted, unavailable — and says nothing
 * about queueing, snapshots, or the cache. Every entry takes what it needs as an
 * argument. Command inputs stay explicit; event emission remains centralized
 * through the window-manager store.
 */

/**
 * Plain-language notices for daemon refusals that arrive as a bare error code.
 * The daemon speaks in codes; the shell's status pill speaks to a person.
 */
const REFUSAL_NOTICES: Readonly<Record<string, string>> = {
  window_manager_revision_conflict: "Layout changed elsewhere. Refreshed.",
  window_manager_invalid_command: "That arrangement isn't possible here.",
  window_manager_invalid_topology: "That arrangement isn't possible here.",
  window_manager_window_pinned: "Unpin the window first.",
  window_manager_final_desktop: "The last desktop stays.",
  window_manager_history_boundary: "Nothing left to undo or redo.",
  window_manager_unavailable: "The saved window layout is unavailable.",
  window_manager_slow_consumer: "The live layout fell behind. Reconnecting.",
};
const GENERIC_REFUSAL_NOTICE = "The window command failed.";

function commandDiagnostic(error: unknown): WindowManagerDiagnosticPayload {
  if (error instanceof WindowManagerApiError && error.payload?.diagnostics[0]) {
    return error.payload.diagnostics[0];
  }
  if (error instanceof WindowManagerApiError) {
    const code = error.payload?.code ?? "command_failed";
    return { code, path: null, message: REFUSAL_NOTICES[code] ?? GENERIC_REFUSAL_NOTICE };
  }
  return {
    code: "command_failed",
    path: null,
    message:
      error instanceof Error && error.message !== "" ? error.message : GENERIC_REFUSAL_NOTICE,
  };
}

/** Reports a command the daemon applied, carrying its first advisory if any. */
export function reportCommandCompleted(
  requestId: string,
  result: WindowManagerCommandResult,
  binding: WindowManagerBinding
): void {
  const firstDiagnostic = result.diagnostics[0];
  windowManagerStore.trigger.commandCompleted({
    commandId: requestId,
    ...(firstDiagnostic
      ? {
          diagnostic: {
            code: firstDiagnostic.code,
            message: firstDiagnostic.message,
            severity: "warning" as const,
            field: firstDiagnostic.path,
          },
        }
      : {}),
    binding,
  });
}

/**
 * Reports a refused command.
 *
 * A revision conflict is a distinct state, not a generic failure: the surface stays
 * read-only until the operator resolves it, so it is recorded with the revisions
 * that disagreed rather than collapsed into an error message.
 */
export function reportCommandRefused(
  requestId: string,
  error: unknown,
  expectedRevision: number,
  binding: WindowManagerBinding
): void {
  const diagnostic = commandDiagnostic(error);
  const storeDiagnostic = {
    code: diagnostic.code,
    message: diagnostic.message,
    severity: "error" as const,
    field: diagnostic.path,
  };
  if (
    error instanceof WindowManagerApiError &&
    error.status === 409 &&
    error.payload?.currentRevision !== null
  ) {
    windowManagerStore.trigger.conflictRecorded({
      conflict: {
        commandId: requestId,
        expectedRevision,
        currentRevision: error.payload?.currentRevision ?? expectedRevision,
      },
      diagnostic: storeDiagnostic,
      binding,
    });
    return;
  }
  windowManagerStore.trigger.commandFailed({
    commandId: requestId,
    diagnostic: storeDiagnostic,
    binding,
  });
}

/** States that this browser cannot command windows until it reconnects. */
export function reportClientUnavailable(): void {
  windowManagerStore.trigger.diagnosticReported({
    diagnostic: {
      code: "client_unavailable",
      message: "Window commands are unavailable until this browser reconnects.",
      severity: "warning",
      field: null,
    },
  });
}

/**
 * Drops the optimistic transition of a failed `desktop.switch`, but only when the
 * live intent still targets this command's desktop — a newer queued switch may have
 * replaced it.
 */
export function clearSwitchTransition(
  command: WindowManagerCommandInput,
  binding: WindowManagerBinding
): void {
  if (command.commandId !== "desktop.switch") return;
  const target = command.payload.desktop_id;
  if (typeof target !== "string") return;
  windowManagerStore.trigger.transitionIntentRejected({ binding, toDesktopId: target });
}
