import type { Page } from "@playwright/test";

import { TERMINAL_SUBPROTOCOL } from "../../src/generated/terminal-wire";
import type { BrowserRuntime } from "./runtime";

const WATCHERS_KEY = "__compozyTerminalE2EWatchers";
const WATCHER_GRIDS_KEY = "__compozyTerminalE2EWatcherGrids";

/**
 * Attaches a read-only watcher and returns the size its ATTACHED frame carried.
 * The socket keeps following RESIZED frames afterwards; `watcherGrid` reads the
 * size the watcher currently holds, which is what a size assertion must compare
 * against once the writer's own window has reflowed around it.
 */
export async function connectTerminalWatcher(
  page: Page,
  runtime: BrowserRuntime,
  workspaceId: string,
  terminalId: string
): Promise<{ cols: number; rows: number }> {
  const ticket = await runtime.requestJSON<{ ticket: string }>(
    `/api/workspaces/${encodeURIComponent(workspaceId)}/terminals/${encodeURIComponent(
      terminalId
    )}/attach-ticket?profile=default`,
    { method: "POST", body: JSON.stringify({ mode: "read" }) }
  );
  return await page.evaluate(
    async input => {
      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const socket = new WebSocket(
        `${protocol}//${window.location.host}/api/workspaces/${encodeURIComponent(
          input.workspaceId
        )}/terminals/${encodeURIComponent(input.terminalId)}/stream?mode=read&flow=drop&ticket=${encodeURIComponent(
          input.ticket
        )}`,
        input.subprotocol
      );
      socket.binaryType = "arraybuffer";
      const attached = await new Promise<{ cols: number; rows: number }>((resolve, reject) => {
        const timeout = window.setTimeout(
          () => reject(new Error("Terminal watcher did not receive its attached frame.")),
          20_000
        );
        socket.onmessage = event => {
          if (!(event.data instanceof ArrayBuffer)) return;
          const bytes = new Uint8Array(event.data);
          // ATTACHED (0x02) answers the connect; RESIZED (0x06) keeps the grid current.
          if (bytes[0] !== 0x02 && bytes[0] !== 0x06) return;
          const payload = JSON.parse(new TextDecoder().decode(bytes.subarray(1))) as {
            cols: number;
            rows: number;
          };
          const grids =
            (Reflect.get(globalThis, input.gridsKey) as
              | Record<string, { cols: number; rows: number }>
              | undefined) ?? {};
          grids[input.terminalId] = { cols: payload.cols, rows: payload.rows };
          Reflect.set(globalThis, input.gridsKey, grids);
          if (bytes[0] !== 0x02) return;
          window.clearTimeout(timeout);
          resolve(payload);
        };
        socket.onerror = () => reject(new Error("Terminal watcher failed to connect."));
      });
      const watchers = (Reflect.get(globalThis, input.key) as WebSocket[] | undefined) ?? [];
      watchers.push(socket);
      Reflect.set(globalThis, input.key, watchers);
      return attached;
    },
    {
      gridsKey: WATCHER_GRIDS_KEY,
      key: WATCHERS_KEY,
      subprotocol: TERMINAL_SUBPROTOCOL,
      ticket: ticket.ticket,
      terminalId,
      workspaceId,
    }
  );
}

/** The size the connected watcher holds right now: its ATTACHED frame or the latest RESIZED. */
export async function watcherGrid(
  page: Page,
  terminalId: string
): Promise<{ cols: number; rows: number } | null> {
  return await page.evaluate(
    input => {
      const grids = Reflect.get(globalThis, input.gridsKey) as
        | Record<string, { cols: number; rows: number }>
        | undefined;
      return grids?.[input.terminalId] ?? null;
    },
    { gridsKey: WATCHER_GRIDS_KEY, terminalId }
  );
}

export async function closeTerminalWatchers(page: Page): Promise<void> {
  await page.evaluate(
    input => {
      const watchers = (Reflect.get(globalThis, input.key) as WebSocket[] | undefined) ?? [];
      for (const watcher of watchers) watcher.close();
      Reflect.deleteProperty(globalThis, input.key);
      Reflect.deleteProperty(globalThis, input.gridsKey);
    },
    { gridsKey: WATCHER_GRIDS_KEY, key: WATCHERS_KEY }
  );
}
