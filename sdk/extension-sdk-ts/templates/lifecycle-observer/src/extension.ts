import { appendFileSync } from "node:fs";

import { Extension } from "@compozy/extension-sdk";
import type { RunPostShutdownPayload } from "@compozy/extension-sdk";

type RecordEntry = {
  kind: string;
  payload?: Record<string, unknown>;
};

export function createLifecycleObserverExtension(
  name = "__EXTENSION_NAME__",
  version = "__EXTENSION_VERSION__"
): Extension {
  return new Extension(name, version).onRunPostShutdown(
    async (_context, payload: RunPostShutdownPayload) => {
      record("run.post_shutdown", {
        run_id: payload.run_id,
        reason: payload.reason,
        status: payload.summary.status,
      });
    }
  );
}

function record(kind: string, payload: Record<string, unknown>): void {
  const path = process.env.COMPOZY_TS_RECORD_PATH?.trim();
  if (path === undefined || path === "") {
    return;
  }

  const entry: RecordEntry = { kind, payload };
  appendFileSync(path, `${JSON.stringify(entry)}\n`, "utf8");
}
