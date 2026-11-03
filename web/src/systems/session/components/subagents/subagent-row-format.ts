// Facts a roster row shows beyond its status: who runs it and what it is doing now.

import {
  firstLine,
  isSubagentLive,
  subagentDisplay,
  subagentRosterGroups,
  type SubagentCounts,
  type SubagentLineTwoTone,
} from "./subagent-format";
import type { SubagentRosterView } from "./subagent-roster";
import type { SubagentView } from "./types";

const CLAUDE_FAMILY =
  /^(?:claude-)?(opus|sonnet|haiku|fable)(?:-(\d+)(?:[-.](\d{1,2}))?)?(?:-\d{8})?(?:\[1m\])?$/i;

/** `claude-sonnet-5-5` and the Agent tool's `sonnet` alias read as names; other ids stay exact. */
export function subagentModelLabel(model: string): string {
  const match = CLAUDE_FAMILY.exec(model.trim());
  if (!match) return model;
  const [, family = "", major, minor] = match;
  const name = `${family[0]?.toUpperCase()}${family.slice(1).toLowerCase()}`;
  return [name, [major, minor].filter(Boolean).join(".")].filter(Boolean).join(" ");
}

/** A provider-native row's `role` is its `subagent_type`; a delegated row names its agent. */
export function subagentNativeKind(subagent: SubagentView): string | null {
  return subagent.origin === "provider_native" ? subagent.role?.trim() || null : null;
}

/** A subagent's recorded runtime as words: the model with its effort, who ran it, and both joined. */
export function subagentRuntimeWords(
  subagent: SubagentView,
  agentLabel: string
): { modelLabel: string; kind: string | null; label: string } {
  const { agent, provider, model, reasoning_effort: effort } = subagent.runtime;
  const kind =
    subagent.origin === "provider_native"
      ? subagentNativeKind(subagent)
      : agent
        ? agentLabel
        : null;
  const modelLabel = model ? [subagentModelLabel(model), effort].filter(Boolean).join(" ") : "";
  return { modelLabel, kind, label: [provider, modelLabel, kind].filter(Boolean).join(" · ") };
}

const TOKEN_FORMAT = new Intl.NumberFormat("en", {
  notation: "compact",
  maximumFractionDigits: 1,
});

export function subagentTokenLabel(tokens: number): string {
  return TOKEN_FORMAT.format(tokens).toLowerCase();
}

export function subagentFinishedLabel(counts: SubagentCounts): string {
  return `${counts.total - counts.live} of ${counts.total} finished`;
}

export interface SubagentLiveLine {
  /** The verb that carries color while it is true: `Running`, `Reading`. */
  verb: string;
  text: string;
  tone: SubagentLineTwoTone;
}

/** The row's third line: the failure, or what a live subagent is doing right now. */
export function subagentLiveLine(
  subagent: SubagentView,
  options: { stale?: boolean } = {}
): SubagentLiveLine | null {
  const display = subagentDisplay(subagent, options);
  if (display.tone === "destructive") {
    const text = firstLine(subagent.error) || firstLine(subagent.result_preview);
    return text ? { verb: "", text, tone: "destructive" } : null;
  }
  if (display.glyph === "stopped") return { verb: "", text: display.word, tone: "default" };
  if (display.tone !== "live" && display.tone !== "attention") return null;
  // A settled child working again stops updating `progress`, so its text would be frozen.
  if (!isSubagentLive(subagent.status)) return { verb: "", text: display.word, tone: "default" };
  const progress = firstLine(subagent.progress);
  if (!progress) return { verb: "", text: display.word, tone: "default" };
  const [, verb = "", rest = ""] = /^(\S+ing)(?:\s+(.*))?$/.exec(progress) ?? [];
  return verb
    ? { verb, text: rest, tone: "default" }
    : { verb: "", text: progress, tone: "default" };
}

/** The list's unfolding, widened so the row a reader comes back to is on the page. */
export function subagentRosterViewWith(
  subagents: readonly SubagentView[],
  view: SubagentRosterView,
  id: string | null
): SubagentRosterView {
  const groups = subagentRosterGroups(subagents);
  const pinned = [...groups.failed, ...groups.live];
  const ordered = [...pinned, ...groups.previous];
  const index = ordered.findIndex(subagent => subagent.id === id);
  if (index < 0) return view;
  return {
    limit: Math.max(view.limit, index + 1),
    previousOpen: view.previousOpen || index >= pinned.length,
  };
}
