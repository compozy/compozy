import { createAtom } from "@xstate/store";
import { toast } from "@compozy/ui";
import type { QueryClient } from "@tanstack/react-query";
import { isRemoteComputerId, normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import type { TerminalInfo } from "@/systems/terminal";
import {
  GLOBAL_DESKTOP_WORKSPACE_ID,
  WINDOW_BINDING_SEARCH_KEY,
  focusedWorkspaceSelector,
  windowDataScope,
} from "@/systems/workspace";
import type { WindowCloseGuard } from "./window-close-targets";

interface CloseConfirmation {
  terminals: readonly TerminalInfo[];
  answer: (confirmed: boolean) => void;
}

/** Tracks one launcher across workspace and destination-profile changes. */
export const terminalWindowCreateKey = (windowId: string) =>
  ["terminal-window-create", windowId] as const;

/**
 * The All projects launcher's create in that window: a child of the window's
 * key, so close admission sees it while the project-window creation state,
 * which reads its own variables under the exact key, does not.
 */
export const terminalAggregateCreateKey = (windowId: string) =>
  [...terminalWindowCreateKey(windowId), "aggregate"] as const;

/** Operator-only orchestration; native window close remains view-only. */
export class TerminalWindowClose {
  readonly confirmation = createAtom<CloseConfirmation | null>(null);

  /** Rejects a pending confirmation when its shell binding is replaced. */
  cancel(): void {
    this.confirmation.get()?.answer(false);
  }

  /** Admits window removal only after every confirmed terminal close has settled successfully. */
  guard(workspaceId: string | null, queryClient: QueryClient): WindowCloseGuard {
    return async (targets, isCurrent) => {
      const windows = targets.filter(window => window.app === "terminal");
      if (windows.length === 0) return true;
      try {
        if (
          windows.some(
            window =>
              // The All projects launcher's create in this window, whatever it shows.
              queryClient.isMutating({ mutationKey: terminalAggregateCreateKey(window.id) }) > 0 ||
              // A project launcher's create; a window already showing a terminal
              // stays closable while it opens another.
              (!window.instanceKey &&
                queryClient.isMutating({
                  mutationKey: terminalWindowCreateKey(window.id),
                  exact: true,
                }) > 0)
          )
        ) {
          throw new Error("The terminal is still opening. Try closing it again when it is ready.");
        }
        const scopes = new Map<
          string,
          { computerId: ComputerId; workspaceId: string; ids: Set<string> }
        >();
        for (const window of windows) {
          if (!window.instanceKey) continue;
          const target = focusedWorkspaceSelector(
            windowDataScope(workspaceId, window.route.search[WINDOW_BINDING_SEARCH_KEY])
          );
          if (!target) {
            // An unbound window under All projects never resolved its terminal to
            // a scope (the aggregate binds every terminal it finds), so there is
            // nothing to terminate from it: close the view, as for terminal_not_found.
            if (workspaceId === GLOBAL_DESKTOP_WORKSPACE_ID) continue;
            throw new Error("Select the terminal's workspace before closing it.");
          }
          const rawComputer = window.route.search.computer;
          const computerId = normalizeComputerId(
            typeof rawComputer === "string" ? rawComputer : undefined
          );
          const scopeKey = `${computerId}\0${target}`;
          const entry = scopes.get(scopeKey) ?? {
            computerId,
            workspaceId: target,
            ids: new Set<string>(),
          };
          entry.ids.add(window.instanceKey);
          scopes.set(scopeKey, entry);
        }
        if (scopes.size === 0) return true;
        // Keep the terminal bundle lazy until an operator actually closes one.
        const api = await import("@/systems/terminal");
        const readTargets = async () =>
          (
            await Promise.all(
              [...scopes.values()].map(async ({ computerId, workspaceId: target, ids }) =>
                (
                  await (isRemoteComputerId(computerId)
                    ? api.fetchTerminals(
                        target,
                        { all_profiles: true },
                        AbortSignal.timeout(15000),
                        computerId
                      )
                    : api.fetchTerminals(
                        target,
                        { all_profiles: true },
                        AbortSignal.timeout(15000)
                      ))
                ).flatMap(terminal =>
                  ids.has(terminal.id) ? [{ computerId, workspaceId: target, terminal }] : []
                )
              )
            )
          ).flat();
        const terminals = await readTargets();
        if (!isCurrent()) return false;
        const running = terminals.filter(({ terminal }) => terminal.state === "running");
        if (running.length > 0) {
          const confirmed = await new Promise<boolean>(resolve => {
            this.confirmation.set({
              terminals: running.map(({ terminal }) => terminal),
              answer: value => {
                this.confirmation.set(null);
                resolve(value);
              },
            });
          });
          if (!confirmed || !isCurrent()) return false;
        }
        // A terminal may exit while the question is open. Re-read rather than
        // treating the dialog's captured status as daemon truth.
        const current = await readTargets();
        if (!isCurrent()) return false;
        const toClose = current.filter(({ terminal }) => terminal.state !== "exited");
        // Validate the complete batch before starting any destructive request.
        if (
          toClose.some(
            ({ computerId, terminal, workspaceId: target }) =>
              !running.some(
                item =>
                  item.computerId === computerId &&
                  item.terminal.id === terminal.id &&
                  item.terminal.profile_name === terminal.profile_name &&
                  item.workspaceId === target
              )
          )
        ) {
          throw new Error("The terminal changed while closing. Try again.");
        }
        // Keep close admission locked until every independent request settles,
        // including after a partial failure, so a retry cannot overlap this batch.
        const results = await Promise.allSettled(
          toClose.map(async ({ computerId, terminal, workspaceId: target }) => {
            try {
              const exit = await (isRemoteComputerId(computerId)
                ? api.closeTerminal(
                    target,
                    terminal.id,
                    { profile: terminal.profile_name },
                    "HUP",
                    AbortSignal.timeout(15000),
                    computerId
                  )
                : api.closeTerminal(
                    target,
                    terminal.id,
                    { profile: terminal.profile_name },
                    "HUP",
                    AbortSignal.timeout(15000)
                  ));
              if (exit === null)
                throw new Error("Terminal termination was not confirmed. Try again.");
            } catch (error) {
              if (
                !(
                  error instanceof api.TerminalApiError && error.domainCode === "terminal_not_found"
                )
              ) {
                throw error;
              }
            }
          })
        );
        const workspaceTargets = new Set([...scopes.values()].map(entry => entry.workspaceId));
        await queryClient.invalidateQueries({
          predicate: query =>
            query.queryKey[0] === "computer" &&
            query.queryKey[2] === "terminal" &&
            (query.queryKey[3] === "aggregate" || workspaceTargets.has(String(query.queryKey[4]))),
        });
        const failure = results.find(result => result.status === "rejected");
        if (failure) throw failure.reason;
        return isCurrent();
      } catch (error) {
        toast.error("Could not close terminal", {
          description: error instanceof Error ? error.message : "Try closing the window again.",
        });
        return false;
      }
    };
  }
}
