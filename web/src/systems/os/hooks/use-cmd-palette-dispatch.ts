import { useQueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";

import { invokeCmdPaletteCommand, recordCmdPaletteUsage } from "../adapters/cmd-palette-api";
import { paletteClientOp, type PaletteShellHandlers } from "../lib/cmd-palette-client-ops";
import {
  dispatchPaletteCommand,
  UNSUPPORTED_CLIENT_OP_REASON,
  type PaletteDispatchOutcome,
} from "../lib/cmd-palette-dispatch";
import type { PaletteRegistry, ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import { cmdPaletteKeys } from "../lib/cmd-palette-query-keys";
import type { OsAppId, OsWindowRoute } from "../lib/os-types";
import { useOsShell } from "./use-os-shell";

export interface UseCmdPaletteDispatchOptions {
  readonly registry: PaletteRegistry;
  readonly workspaceId: string | null;
  readonly shell: PaletteShellHandlers;
  /** Opens an OS app window; the projection's `navigate` actions land here. */
  readonly openApp: (app: OsAppId, route: OsWindowRoute | null) => void;
}

export interface CmdPaletteDispatch {
  /** Runs a resolved command through the seam. */
  run(
    command: ResolvedPaletteCommand,
    args?: Readonly<Record<string, unknown>>,
    query?: string
  ): Promise<PaletteDispatchOutcome>;
  /** Runs a command by id — the keyboard and menubar entry point. */
  runById(
    commandId: string,
    args?: Readonly<Record<string, unknown>>
  ): Promise<PaletteDispatchOutcome>;
  /**
   * Executes a `client_op` the daemon pushed over the window-manager channel.
   * Same table as every local path, so an agent's invoke and ⌘W cannot diverge.
   */
  executeClientOp(op: string, payload: unknown): Promise<unknown>;
}

/**
 * Binds the dispatch seam to this client's live coordinators.
 *
 * Everything that can run a command routes through the returned object, so
 * availability refusal, routing and failure reporting happen once rather than
 * per surface.
 */
export function useCmdPaletteDispatch({
  registry,
  workspaceId,
  shell,
  openApp,
}: UseCmdPaletteDispatchOptions): CmdPaletteDispatch {
  const { manager } = useOsShell();
  const queryClient = useQueryClient();

  const runCommand = (
    command: ResolvedPaletteCommand,
    args?: Readonly<Record<string, unknown>>,
    query = ""
  ): Promise<PaletteDispatchOutcome> =>
    dispatchPaletteCommand({
      command,
      args,
      query,
      ports: {
        clientOps: { manager, shell },
        invoke: async (commandId, invokeArgs) => {
          if (workspaceId === null) {
            throw new Error("This workspace is still connecting. Try again in a moment.");
          }
          const result = await invokeCmdPaletteCommand({
            workspaceId,
            commandId,
            args: invokeArgs,
          });
          void queryClient.invalidateQueries({
            queryKey: cmdPaletteKeys.rankSignals(workspaceId),
          });
          return result;
        },
        navigate: (app, pathname) =>
          openApp(app as OsAppId, pathname === null ? null : { pathname, search: {} }),
        pushView: viewId => shell.openPaletteView(viewId),
        openUrl: url => window.open(url, "_blank", "noopener,noreferrer"),
        reportUsage: (commandId, preselectionQuery) => {
          if (workspaceId === null) return;
          void recordCmdPaletteUsage(workspaceId, commandId, preselectionQuery)
            .then(() =>
              queryClient.invalidateQueries({
                queryKey: cmdPaletteKeys.rankSignals(workspaceId),
              })
            )
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
        onFailure: (failed, reason) =>
          notifyUser({ message: `${failed.title} — ${reason}`, tone: "error" }),
      },
    });

  const runById = async (
    commandId: string,
    args?: Readonly<Record<string, unknown>>
  ): Promise<PaletteDispatchOutcome> => {
    const command = registry.byId.get(commandId);
    if (command === undefined) {
      // Truthful UI: a chord or menu item pointing at a command the catalog no
      // longer carries reports that rather than doing nothing.
      return { status: "refused", reason: UNSUPPORTED_CLIENT_OP_REASON };
    }
    return await runCommand(command, args);
  };

  const executeClientOp = async (op: string, payload: unknown) => {
    const handler = paletteClientOp(op);
    if (handler === null) throw new Error(`Unsupported client operation: ${op}`);
    return await handler({ manager, shell }, payload);
  };

  return { run: runCommand, runById, executeClientOp };
}
