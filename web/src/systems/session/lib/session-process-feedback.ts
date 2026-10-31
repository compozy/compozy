import { toast } from "@compozy/ui";

import type { SessionProcessResult } from "../adapters/session-processes-api";
import {
  processCommandLabel,
  processIdentity,
  processStopReport,
  processTreeMembers,
  type SessionProcessTree,
} from "./session-processes";

const SETTLED = new Set<SessionProcessResult["outcome"]>(["terminated", "already_exited"]);

/**
 * The daemon answers once per process; the person stopped trees. Each tree reads as one result:
 * stopped when every member settled, else its first refusal. A result no listed tree names stays.
 */
export function processTreeResults(
  results: readonly SessionProcessResult[],
  trees: readonly SessionProcessTree[]
): { result: SessionProcessResult; tree?: SessionProcessTree }[] {
  const treeOf = new Map<string, SessionProcessTree>();
  for (const tree of trees) {
    for (const member of processTreeMembers(tree)) treeOf.set(processIdentity(member), tree);
  }
  const byTree = new Map<string, { result: SessionProcessResult; tree?: SessionProcessTree }>();
  const loose: { result: SessionProcessResult }[] = [];
  for (const result of results) {
    const tree = treeOf.get(processIdentity(result));
    if (!tree) {
      loose.push({ result });
      continue;
    }
    const current = byTree.get(tree.id);
    const lead = { ...result, pid: tree.lead.pid, started_at: tree.lead.started_at };
    if (!current) {
      byTree.set(tree.id, { result: lead, tree });
    } else if (SETTLED.has(current.result.outcome) && !SETTLED.has(result.outcome)) {
      byTree.set(tree.id, { result: lead, tree });
    } else if (current.result.outcome === "already_exited" && result.outcome === "terminated") {
      byTree.set(tree.id, { result: lead, tree });
    }
  }
  return [...byTree.values(), ...loose];
}

/** One toast per stop, naming each tree the daemon refused by its lead command. */
export function reportProcessStop(
  results: readonly SessionProcessResult[],
  trees: readonly SessionProcessTree[]
): void {
  const grouped = processTreeResults(results, trees);
  const commands = new Map(
    grouped.map(({ result, tree }) => [
      processIdentity(result),
      tree ? processCommandLabel(tree.lead) : `pid ${result.pid}`,
    ])
  );
  const report = processStopReport(
    grouped.map(({ result }) => result),
    result => commands.get(processIdentity(result)) ?? `pid ${result.pid}`
  );
  const options = report.description ? { description: report.description } : undefined;
  if (report.tone === "success") toast.success(report.title, options);
  else if (report.tone === "info") toast.info(report.title, options);
  else toast.error(report.title, options);
}
