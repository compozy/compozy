import { toast } from "sonner";

import type {
  SessionProcessPayload,
  SessionProcessResult,
} from "../adapters/session-processes-api";
import { processCommandLabel, processIdentity, processStopReport } from "./session-processes";

/** One toast per stop, naming each process the daemon refused by its command. */
export function reportProcessStop(
  results: readonly SessionProcessResult[],
  listed: readonly SessionProcessPayload[]
): void {
  const byIdentity = new Map(listed.map(process => [processIdentity(process), process]));
  const report = processStopReport(results, result => {
    const process = byIdentity.get(processIdentity(result));
    return process ? processCommandLabel(process) : `pid ${result.pid}`;
  });
  const options = report.description ? { description: report.description } : undefined;
  if (report.tone === "success") toast.success(report.title, options);
  else if (report.tone === "info") toast.info(report.title, options);
  else toast.error(report.title, options);
}
