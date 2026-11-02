// Facts a roster row shows beyond its status: who runs it and what it is doing now.

import {
  firstLine,
  subagentDisplay,
  type SubagentCounts,
  type SubagentLineTwoTone,
} from "./subagent-format";
import type { SubagentView } from "./types";

const CLAUDE_FAMILY = /^(?:claude-)?(opus|sonnet|haiku|fable)(?:-(\d+))?(?:[-.](\d{1,2}))?\b/i;

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
  if (subagent.status === "failed") {
    const text = firstLine(subagent.error);
    return text ? { verb: "", text, tone: "destructive" } : null;
  }
  const display = subagentDisplay(subagent, options);
  if (display.glyph === "stopped") return { verb: "", text: display.word, tone: "default" };
  if (display.tone !== "live" && display.tone !== "attention") return null;
  const progress = firstLine(subagent.progress);
  if (!progress) return { verb: "", text: display.word, tone: "default" };
  const [, verb = "", rest = ""] = /^(\S+ing)(?:\s+(.*))?$/.exec(progress) ?? [];
  return verb
    ? { verb, text: rest, tone: "default" }
    : { verb: "", text: progress, tone: "default" };
}
