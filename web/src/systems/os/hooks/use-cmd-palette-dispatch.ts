import { useQueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";

import {
  invokeCmdPaletteCommand,
  recordCmdPaletteUsage,
  setCmdPalettePin,
} from "../adapters/cmd-palette-api";
import { paletteClientOp, type PaletteShellHandlers } from "../lib/cmd-palette-client-ops";
import { writePaletteClipboard } from "../lib/cmd-palette-copy";
import {
  dispatchPaletteCommand,
  STALE_TARGET_REASON,
  type PaletteDispatchOutcome,
} from "../lib/cmd-palette-dispatch";
import {
  invokeCompletedFeedback,
  invokeFailedFeedback,
  type PaletteFeedback,
} from "../lib/cmd-palette-feedback";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import { cmdPaletteKeys } from "../lib/cmd-palette-query-keys";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import {
  cmdPaletteExecutionStore,
  requestPaletteArgs,
  requestPaletteConfirmation,
} from "../stores/cmd-palette-execution-store";
import { windowManagerStore } from "../stores/window-manager-store";
import { useOsShell } from "./use-os-shell";

/**
 * Delivers one outcome. Retry rides the toast only where the feedback says
 * re-running is safe, so the button cannot outlive the judgement that allowed it.
 */
function announce(feedback: PaletteFeedback, retry?: () => void): void {
  notifyUser({
    message: feedback.message,
    tone: feedback.tone,
    ...(feedback.retryable && retry ? { action: { label: "Retry", onClick: retry } } : {}),
  });
}

function openExternalUrl(url: string): void {
  window.open(url, "_blank", "noopener,noreferrer");
}

export interface UseCmdPaletteDispatchOptions {
  readonly registry: PaletteRegistry;
  readonly workspaceId: string | null;
  readonly shell: PaletteShellHandlers;
  /** Opens an OS app window; the projection's `navigate` actions land here. */
  readonly openApp: (app: OsAppId, route: OsWindowRoute | null) => void;
  /** Explicit invoke target; the bound window-manager client is the fallback. */
  readonly clientId?: string | null;
  /** Current attachment token; omitted so control-plane invokes send no header. */
  readonly attachmentToken?: string | null;
}

export interface CmdPaletteRunOptions {
  readonly args?: Readonly<Record<string, unknown>>;
  readonly query?: string;
  /** Set once the operator cleared the command's declared confirmation. */
  readonly confirmed?: boolean;
}

export interface CmdPaletteDispatch {
  /** Runs a resolved command through the seam. */
  run(
    command: ResolvedPaletteCommand,
    options?: CmdPaletteRunOptions
  ): Promise<PaletteDispatchOutcome>;
  /** Runs a command by id — the keyboard and menubar entry point. */
  runById(commandId: string, options?: CmdPaletteRunOptions): Promise<PaletteDispatchOutcome>;
  /**
   * Executes a `client_op` the daemon pushed over the window-manager channel.
   * Same table as every local path, so an agent's invoke and ⌘W cannot diverge.
   */
  executeClientOp(op: string, payload: unknown): Promise<unknown>;
  /** The action panel's Pin/Unpin meta-action; idempotent on the daemon side. */
  setPinned(command: ResolvedPaletteCommand, pinned: boolean): Promise<void>;
}

/**
 * Binds the dispatch seam to this client's live coordinators.
 *
 * Everything that can run a command routes through the returned object, so
 * availability refusal, routing and failure reporting happen once rather than
 * per surface. Feedback rides `notifyUser`, which outlives the palette — an
 * invocation the operator started and then dismissed still reports (US-017.AC-2).
 */
export function resolveInvokeClientId(
  explicit?: string | null,
  boundClientId = windowManagerStore.getSnapshot().context.binding?.clientId
): string | undefined {
  const fromOptions = explicit?.trim();
  if (fromOptions) return fromOptions;
  const bound = boundClientId?.trim();
  return bound === undefined || bound === "" ? undefined : bound;
}

export function resolveInvokeAttachmentToken(explicit?: string | null): string | undefined {
  const token = explicit?.trim();
  return token === undefined || token === "" ? undefined : token;
}

export function useCmdPaletteDispatch({
  registry,
  workspaceId,
  shell,
  openApp,
  clientId,
  attachmentToken,
}: UseCmdPaletteDispatchOptions): CmdPaletteDispatch {
  const { manager } = useOsShell();
  const queryClient = useQueryClient();

  const invalidateRankSignals = () => {
    if (workspaceId === null) return;
    void queryClient.invalidateQueries({ queryKey: cmdPaletteKeys.rankSignals(workspaceId) });
  };

  const navigate = (app: string, pathname: string | null) =>
    openApp(app as OsAppId, pathname === null ? null : { pathname, search: {} });
  const clientOps = { manager, shell, navigate, openUrl: openExternalUrl };

  const runCommand = (
    command: ResolvedPaletteCommand,
    options: CmdPaletteRunOptions = {}
  ): Promise<PaletteDispatchOutcome> => {
    const { args, query = "", confirmed = false } = options;
    // A retry re-enters the seam from the top, so a command that declares a
    // confirmation asks again rather than firing straight off a toast button.
    const retry = () => void runCommand(command, { ...(args ? { args } : {}), query });
    return dispatchPaletteCommand({
      command,
      ...(args ? { args } : {}),
      query,
      confirmed,
      ports: {
        clientOps,
        invoke: async (commandId, invokeArgs) => {
          if (workspaceId === null) {
            throw new Error("This workspace is still connecting. Try again in a moment.");
          }
          const client = resolveInvokeClientId(clientId);
          const token = resolveInvokeAttachmentToken(attachmentToken);
          const result = await invokeCmdPaletteCommand({
            workspaceId,
            commandId,
            args: invokeArgs,
            ...(client === undefined ? {} : { clientId: client }),
            ...(token === undefined ? {} : { attachmentToken: token }),
          });
          invalidateRankSignals();
          return result;
        },
        navigate,
        pushView: viewId => shell.openPaletteView(viewId),
        openUrl: openExternalUrl,
        copyToClipboard: writePaletteClipboard,
        reportUsage: (commandId, preselectionQuery) => {
          if (workspaceId === null) return;
          void recordCmdPaletteUsage(workspaceId, commandId, preselectionQuery)
            .then(invalidateRankSignals)
            .catch(error => {
              console.warn("Failed to record command palette usage", error);
            });
        },
        refresh: () => {
          if (workspaceId === null) return;
          void queryClient.invalidateQueries({
            queryKey: cmdPaletteKeys.workspaceCatalogs(workspaceId),
          });
        },
        onFailure: (failed, reason, code) =>
          announce(invokeFailedFeedback(failed, reason, code), retry),
        // Raising the overlay clears any previous step first, so the intent is
        // written after it — a fresh palette must not drop the step it opened for.
        requestArgs: target => {
          shell.openPaletteExecution();
          requestPaletteArgs(target);
        },
        requestConfirmation: (target, collected) => {
          shell.openPaletteExecution();
          requestPaletteConfirmation(target, collected);
        },
        onPendingStart: target =>
          cmdPaletteExecutionStore.trigger.pendingStarted({
            pending: { commandId: target.id, title: target.title },
          }),
        onPendingSettle: target =>
          cmdPaletteExecutionStore.trigger.pendingSettled({ commandId: target.id }),
        onCompleted: (target, result) => announce(invokeCompletedFeedback(target, result)),
      },
    });
  };

  const runById = async (
    commandId: string,
    options?: CmdPaletteRunOptions
  ): Promise<PaletteDispatchOutcome> => {
    const command = registry.byId.get(commandId);
    if (command === undefined) {
      // Truthful UI: a chord or menu item pointing at a command the catalog no
      // longer carries reports that rather than doing nothing.
      announce({ message: STALE_TARGET_REASON, tone: "error", retryable: false });
      return { status: "refused", reason: STALE_TARGET_REASON };
    }
    return await runCommand(command, options);
  };

  const executeClientOp = async (op: string, payload: unknown) => {
    const handler = paletteClientOp(op);
    if (handler === null) throw new Error(`Unsupported client operation: ${op}`);
    return await handler(clientOps, payload);
  };

  const setPinned = async (command: ResolvedPaletteCommand, pinned: boolean) => {
    if (workspaceId === null) {
      notifyUser({
        message: "This workspace is still connecting. Try again in a moment.",
        tone: "error",
      });
      return;
    }
    try {
      await setCmdPalettePin(workspaceId, command.id, pinned);
    } catch (error) {
      const reason = error instanceof Error ? error.message : "could not be saved";
      notifyUser({ message: `${command.title} — ${reason}`, tone: "error" });
      return;
    }
    // Pins live in the rank-signal snapshot and change the catalog revision, so
    // both reads reconcile rather than the row guessing its own new state.
    invalidateRankSignals();
    void queryClient.invalidateQueries({
      queryKey: cmdPaletteKeys.workspaceCatalogs(workspaceId),
    });
  };

  return { run: runCommand, runById, executeClientOp, setPinned };
}
