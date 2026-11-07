import type { DesktopLogger } from "../logging/desktop-logger";
import type { ShellSnapshot } from "../state/app-state";
import { RuntimeHealthMonitor } from "./runtime-health-monitor";

export interface ProductRuntimeShell {
  /** True once the shell is quitting; no reconnect work may start after that. */
  readonly stopped: () => boolean;
  readonly publish: (snapshot: ShellSnapshot) => Promise<void>;
  readonly reloadProduct: () => Promise<void>;
  /** Shows the boot window. It must exist before the product window closes, or the app quits. */
  readonly showBoot: () => void;
  /** Renders a snapshot in the boot window without waiting for it to be persisted. */
  readonly renderBoot: (snapshot: ShellSnapshot) => void;
  /**
   * Destroys the product window (even if its state cannot be saved) and queues its route after
   * the close resets the deep-link queue, so the replacement window reopens it.
   */
  readonly closeProduct: () => Promise<void>;
  /** Attaches to or starts the runtime again and opens a new product window. */
  readonly resolveRuntime: () => Promise<void>;
  readonly logger: Pick<DesktopLogger, "info" | "error">;
  /** The desktop log a recovery failure points the user to. */
  readonly logPath: string;
}

/**
 * Keeps the product window bound to its runtime across daemon restarts. A settings restart or
 * runtime update replaces the daemon on the same origin: the window stays open and reloads its
 * route once the replacement answers. A runtime that never returns hands off to the boot window,
 * which resolves the runtime again and offers retry on failure.
 */
export function monitorProductRuntime(
  runtime: { readonly origin: string; readonly owned: boolean },
  shell: ProductRuntimeShell,
  monitorOptions: Partial<
    Pick<
      ConstructorParameters<typeof RuntimeHealthMonitor>[0],
      "fetcher" | "intervalMs" | "timeoutMs" | "failureThreshold" | "reconnectTimeoutMs" | "now"
    >
  > = {}
): RuntimeHealthMonitor {
  const fields = { origin: runtime.origin };
  return new RuntimeHealthMonitor({
    ...monitorOptions,
    origin: runtime.origin,
    onError: error => shell.logger.error("follow product runtime", error),
    onDisconnected: async () => {
      if (shell.stopped()) return;
      shell.logger.info("runtime unreachable; awaiting reconnect", fields);
      await shell.publish({ state: "disconnected" });
    },
    onReconnected: async () => {
      if (shell.stopped()) return;
      shell.logger.info("runtime reconnected; reloading product window", fields);
      try {
        await shell.publish({ state: "product", origin: runtime.origin, owned: runtime.owned });
      } catch (error) {
        // The app record is advisory and the next state change rewrites it; reload regardless.
        shell.logger.error("publish reconnected runtime state", error);
      }
      await shell.reloadProduct();
    },
    onLost: async () => {
      if (shell.stopped()) return;
      shell.logger.info("runtime did not return; resolving it again", fields);
      try {
        await shell.publish({ state: "disconnected" });
      } catch (error) {
        // The snapshot is advisory; a failed write must not strand the handoff below.
        shell.logger.error("publish lost runtime state", error);
      }
      if (shell.stopped()) return;
      shell.showBoot();
      try {
        await shell.closeProduct();
      } catch (error) {
        // Without this, the boot page would stay "disconnected", which offers no Retry.
        shell.logger.error("close product window for runtime recovery", error);
        const failure = {
          state: "error",
          error: {
            code: "runtime_recovery_failed",
            safe_message: "CompozyOS stopped responding and could not reconnect.",
            log_path: shell.logPath,
          },
        } as const;
        // The boot window shows Retry even when the app record cannot be written.
        shell.renderBoot(failure);
        try {
          await shell.publish(failure);
        } catch (publishError) {
          shell.logger.error("publish runtime recovery failure", publishError);
        }
        return;
      }
      await shell.resolveRuntime();
    },
  });
}
