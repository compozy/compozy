import type { Page } from "@playwright/test";

import { TERMINAL_SUBPROTOCOL } from "../../src/generated/terminal-wire";
import type { BrowserRuntime } from "./runtime";

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
          if (bytes[0] !== 0x02) return;
          window.clearTimeout(timeout);
          const payload = JSON.parse(new TextDecoder().decode(bytes.subarray(1))) as {
            cols: number;
            rows: number;
          };
          resolve(payload);
        };
        socket.onerror = () => reject(new Error("Terminal watcher failed to connect."));
      });
      const key = "__compozyTerminalE2EWatchers";
      const watchers = (Reflect.get(globalThis, key) as WebSocket[] | undefined) ?? [];
      watchers.push(socket);
      Reflect.set(globalThis, key, watchers);
      return attached;
    },
    { subprotocol: TERMINAL_SUBPROTOCOL, ticket: ticket.ticket, terminalId, workspaceId }
  );
}

export async function closeTerminalWatchers(page: Page): Promise<void> {
  await page.evaluate(() => {
    const key = "__compozyTerminalE2EWatchers";
    const watchers = (Reflect.get(globalThis, key) as WebSocket[] | undefined) ?? [];
    for (const watcher of watchers) watcher.close();
    Reflect.deleteProperty(globalThis, key);
  });
}
