import { clipboard } from "electron";

import type { AppStatePublisher } from "../state/app-state";
import { ControlMethodError, type ControlHandler } from "./control-contract";

interface ControlActions {
  readonly navigate: (path: string) => Promise<string>;
  readonly retry: () => Promise<void>;
  readonly exportDiagnostics: () => Promise<{
    readonly bundle_path: string;
    readonly bytes: number;
  }>;
}

function recordParams(params: unknown): Record<string, unknown> {
  if (!params || typeof params !== "object" || Array.isArray(params)) return {};
  return params as Record<string, unknown>;
}

export function createControlHandler(
  publisher: AppStatePublisher,
  actions: ControlActions
): ControlHandler {
  return async (method, params) => {
    switch (method) {
      case "navigate": {
        const path = recordParams(params).path;
        if (typeof path !== "string") {
          throw new ControlMethodError("invalid_target_path", "A product path is required.");
        }
        return { navigated: await actions.navigate(path) };
      }
      case "retry":
        if (publisher.snapshot().state !== "error") return { ok: true };
        await actions.retry();
        return { accepted: true };
      case "diagnose":
        return publisher.diagnosticReport();
      case "copy_diagnostics": {
        const report = publisher.diagnosticReport();
        clipboard.writeText(JSON.stringify(report, null, 2));
        return { copied: true };
      }
      case "export_diagnostics":
        if (recordParams(params).consent !== true) {
          throw new Error("Diagnostic bundle consent is required.");
        }
        return await actions.exportDiagnostics();
    }
  };
}
