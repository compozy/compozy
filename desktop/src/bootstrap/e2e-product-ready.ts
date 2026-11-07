import { access } from "node:fs/promises";
import { setTimeout as delay } from "node:timers/promises";

/** Holds product-window creation until a desktop E2E run releases its ready gate file. */
export async function waitForE2EProductReady(signal: AbortSignal): Promise<void> {
  const readyPath = process.env.COMPOZY_DESKTOP_E2E_READY_FILE?.trim();
  if (process.env.COMPOZY_DESKTOP_E2E !== "1" || !readyPath) return;
  const deadline = Date.now() + 10_000;
  while (Date.now() < deadline) {
    signal.throwIfAborted();
    try {
      await access(readyPath);
      return;
    } catch (error) {
      if (!(error instanceof Error && "code" in error && error.code === "ENOENT")) throw error;
    }
    await delay(25, undefined, { signal });
  }
  throw new Error("The desktop E2E product-ready gate timed out.");
}
