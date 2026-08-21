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

function commandDiagnostic(error: unknown): WindowManagerDiagnosticPayload {
  if (error instanceof WindowManagerApiError && error.payload?.diagnostics[0]) {
    return error.payload.diagnostics[0];
  }
  return {
    code:
      error instanceof WindowManagerApiError
        ? (error.payload?.code ?? "command_failed")
        : "command_failed",
    path: null,
    message: error instanceof Error ? error.message : "The window command failed.",
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
