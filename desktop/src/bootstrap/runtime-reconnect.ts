import type { DesktopLogger } from "../logging/desktop-logger";
import type { ShellSnapshot } from "../state/app-state";
import { RuntimeHealthMonitor, type RuntimeIdentity } from "./runtime-health-monitor";
import { runtimeProcessAlive } from "./runtime-liveness";

export interface ProductRuntimeShell {
  /** True once the shell is quitting; no reconnect work may start after that. */
  readonly stopped: () => boolean;
  readonly publish: (snapshot: ShellSnapshot) => Promise<void>;
  readonly reloadProduct: () => Promise<void>;
  /**
   * Whether the product window still shows a loaded page of this runtime. Only such a page can
   * refresh itself after a reconnect; absent, every reconnect reloads.
   */
  readonly productLoaded?: () => boolean;
  /** The CompozyOS home whose lock files name runtime processes; absent, none counts as alive. */
  readonly home?: string;
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

const STATUS_TIMEOUT_MS = 3_000;

/** A version without its optional `v`, so the status and identity spellings compare equal. */
function releaseVersion(version: string): string {
  return version.trim().replace(/^v/u, "");
}

type MonitorOptions = Partial<
  Pick<
    ConstructorParameters<typeof RuntimeHealthMonitor>[0],
    | "fetcher"
    | "intervalMs"
    | "timeoutMs"
    | "failureThreshold"
    | "reconnectTimeoutMs"
    | "maximumWaitMs"
    | "runtimeAlive"
    | "now"
  >
>;

/**
 * Whether the runtime's own web refreshes itself after a reconnect. The web that does ships with
 * the runtimes that report a lifecycle phase, so the phase's presence is the capability.
 */
async function refreshesItself(
  origin: string,
  fetcher: NonNullable<MonitorOptions["fetcher"]>
): Promise<boolean> {
  try {
    const response = await fetcher(new URL("/api/status", origin).toString(), {
      method: "GET",
      signal: AbortSignal.timeout(STATUS_TIMEOUT_MS),
    });
    if (!response.ok) return false;
    const body: unknown = await response.json();
    const daemon = (body as { daemon?: { readiness?: { phase?: unknown } } } | null)?.daemon;
    return typeof daemon?.readiness?.phase === "string";
  } catch {
    return false;
  }
}

/**
 * Keeps the product window bound to its runtime across daemon restarts. A settings restart or
 * runtime update replaces the daemon on the same origin: the window stays open and, once the
 * replacement answers, the loaded page refreshes its own data. The window reloads instead when
 * the page cannot do that: the runtime now serves another version's web bundle, the page did not
 * load, or the runtime predates self-refresh. A runtime whose process is gone hands off to the
 * boot window, which resolves the runtime again and offers retry on failure.
 */
export function monitorProductRuntime(
  runtime: {
    readonly origin: string;
    readonly owned: boolean;
    readonly version?: string;
    readonly pid?: number;
  },
  shell: ProductRuntimeShell,
  monitorOptions: MonitorOptions = {}
): RuntimeHealthMonitor {
  const fields = { origin: runtime.origin };
  const home = shell.home;
  /** True when the loaded page is this runtime version's own and can refresh its data itself. */
  const keepsPage = async (identity: RuntimeIdentity | null): Promise<boolean> =>
    identity !== null &&
    runtime.version !== undefined &&
    releaseVersion(identity.version) === releaseVersion(runtime.version) &&
    shell.productLoaded?.() === true &&
    (await refreshesItself(runtime.origin, monitorOptions.fetcher ?? fetch));
  return new RuntimeHealthMonitor({
    ...(home === undefined ? {} : { runtimeAlive: () => runtimeProcessAlive(home, runtime.pid) }),
    ...monitorOptions,
    origin: runtime.origin,
    onError: error => shell.logger.error("follow product runtime", error),
    onDisconnected: async () => {
      if (shell.stopped()) return;
      shell.logger.info("runtime unreachable; awaiting reconnect", fields);
      await shell.publish({ state: "disconnected" });
    },
    onReconnected: async identity => {
      if (shell.stopped()) return;
      const soft = await keepsPage(identity);
      if (shell.stopped()) return;
      shell.logger.info(
        soft
          ? "runtime reconnected; product window refreshes its data"
          : "runtime reconnected; reloading product window",
        fields
      );
      try {
        await shell.publish({ state: "product", origin: runtime.origin, owned: runtime.owned });
      } catch (error) {
        // The app record is advisory and the next state change rewrites it; reload regardless.
        shell.logger.error("publish reconnected runtime state", error);
      }
      if (!soft) await shell.reloadProduct();
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
