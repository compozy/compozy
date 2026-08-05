import type { AssistantState } from "@assistant-ui/react";

import { isAgentEventPayload, parseToolUseResult } from "../lib/message-parts";
import type { CostDisplay } from "@/lib/cost-provenance";

export type ThreadMessageState = AssistantState["thread"]["messages"][number];

export interface InspectorFileEntry {
  path: string;
  readCount: number;
}

/** Aggregate tool messages by file path while preserving first-seen order. */
export function deriveFileReads(messages: readonly ThreadMessageState[]): InspectorFileEntry[] {
  const index = new Map<string, InspectorFileEntry>();
  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }

    for (const part of message.parts) {
      if (part.type !== "tool-call") {
        continue;
      }

      const result = isAgentEventPayload(part.result) ? parseToolUseResult(part.result) : null;
      const path = result?.filePath ?? readFilePathFromInput(part.args);
      if (!path) {
        continue;
      }

      const existing = index.get(path);
      if (existing) {
        existing.readCount += 1;
      } else {
        index.set(path, { path, readCount: 1 });
      }
    }
  }
  return Array.from(index.values());
}

function readFilePathFromInput(input: Record<string, unknown> | undefined): string | undefined {
  if (!input) {
    return undefined;
  }
  const raw = input.file_path ?? input.filePath ?? input.path;
  return typeof raw === "string" && raw.length > 0 ? raw : undefined;
}

/** Token/turn fields the Usage-presence predicate reads (structural subset of InspectorUsage). */
interface UsagePresenceFields {
  tokensIn?: number;
  tokensOut?: number;
  totalTokens?: number;
  turnCount?: number;
}

/**
 * The Usage panel opens for any reported token counter, an authoritative cost
 * status (`cost.hasCost`, owned by describeCost), or a positive turn count. A
 * statusless amount alone never opens it: describeCost reports `hasCost: false`
 * for it, so this predicate excludes it by construction.
 */
export function hasReportableUsage(usage: UsagePresenceFields, cost: CostDisplay): boolean {
  return (
    usage.tokensIn !== undefined ||
    usage.tokensOut !== undefined ||
    usage.totalTokens !== undefined ||
    cost.hasCost ||
    (usage.turnCount ?? 0) > 0
  );
}
