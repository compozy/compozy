import { createAtom } from "@xstate/store";
import { toast } from "@compozy/ui";
import type { QueryClient } from "@tanstack/react-query";
import type { TerminalInfo } from "@/systems/terminal";
import type { WindowCloseGuard } from "./window-close-targets";

interface CloseConfirmation {
  terminals: readonly TerminalInfo[];
  answer: (confirmed: boolean) => void;
}

/** Tracks launcher creation across destination-profile changes. */
export const terminalWindowCreateKey = (workspaceId: string, windowId: string) =>
  ["terminal-window-create", workspaceId, windowId] as const;

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
      if (!workspaceId && windows.every(window => !window.instanceKey)) return true;
      try {
        if (!workspaceId) throw new Error("Select the terminal's workspace before closing it.");
        if (
          windows.some(
            window =>
              !window.instanceKey &&
              queryClient.isMutating({
                mutationKey: terminalWindowCreateKey(workspaceId, window.id),
                exact: true,
              }) > 0
          )
        ) {
          throw new Error("The terminal is still opening. Try closing it again when it is ready.");
        }
        // Keep the terminal bundle lazy until an operator actually closes one.
        const api = await import("@/systems/terminal");
        const ids = new Set(windows.map(window => window.instanceKey));
        const readTargets = async () =>
          (
            await api.fetchTerminals(
              workspaceId,
              { all_profiles: true },
              AbortSignal.timeout(15000)
            )
          ).filter(terminal => ids.has(terminal.id));
        const terminals = await readTargets();
        if (!isCurrent()) return false;
        const running = terminals.filter(terminal => terminal.state === "running");
        if (running.length > 0) {
          const confirmed = await new Promise<boolean>(resolve => {
            this.confirmation.set({
              terminals: running,
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
        const toClose = current.filter(terminal => terminal.state !== "exited");
        // Validate the complete batch before starting any destructive request.
        if (
          toClose.some(
            terminal =>
              !running.some(
                item => item.id === terminal.id && item.profile_name === terminal.profile_name
              )
          )
        ) {
          throw new Error("The terminal changed while closing. Try again.");
        }
        // Keep close admission locked until every independent request settles,
        // including after a partial failure, so a retry cannot overlap this batch.
        const results = await Promise.allSettled(
          toClose.map(async terminal => {
            try {
              const exit = await api.closeTerminal(
                workspaceId,
                terminal.id,
                { profile: terminal.profile_name },
                "HUP",
                AbortSignal.timeout(15000)
              );
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
        await queryClient.invalidateQueries({
          predicate: query => query.queryKey[0] === "terminal" && query.queryKey[2] === workspaceId,
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
