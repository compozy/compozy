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
      await shell.publish({ state: "product", origin: runtime.origin, owned: runtime.owned });
      await shell.reloadProduct();
    },
    onLost: async () => {
      if (shell.stopped()) return;
      shell.logger.info("runtime did not return; resolving it again", fields);
      await shell.publish({ state: "disconnected" });
      shell.showBoot();
      try {
        await shell.closeProduct();
      } catch (error) {
        // Without this, the boot page would stay "disconnected", which offers no Retry.
        shell.logger.error("close product window for runtime recovery", error);
        await shell.publish({
          state: "error",
          error: {
            code: "runtime_recovery_failed",
            safe_message: "CompozyOS stopped responding and could not reconnect.",
            log_path: shell.logPath,
          },
        });
        return;
      }
      await shell.resolveRuntime();
    },
  });
}
