import {
  isLoopDiffChange,
  LOOP_DIFF_CHANGE_LABEL,
  LOOP_DIFF_CHANGE_TONE,
  LOOP_DIFF_CHANGES,
  type LoopDiffChange,
} from "./loop-request-vocabulary";
import { isTerminalLoopStatus } from "./loop-formatters";
import type { LoopDiff, LoopDiffInput, LoopDiffNode, LoopDiffValue } from "../types";
import type { PillTone } from "@compozy/ui";

export interface LoopDiffValueView {
  text: string;

  summary: string;
  isSummarized: boolean;
  isAbsent: boolean;
}

export interface LoopDiffRowView {
  key: string;
  nodeId: string;
  itemIndex: number | null;
  change: LoopDiffChange;
  changeLabel: string;
  tone: PillTone;

  cause: string;
  base: LoopDiffValueView;
  against: LoopDiffValueView;
}

export interface LoopDiffGroupView {
  change: LoopDiffChange;
  label: string;
  tone: PillTone;
  rows: LoopDiffRowView[];
}

export interface LoopDiffInputRowView {
  key: string;
  base: LoopDiffValueView;
  against: LoopDiffValueView;
  changed: boolean;
}

export interface LoopDiffView {
  kind: string;
  isRunCompare: boolean;
  groups: LoopDiffGroupView[];
  inputs: LoopDiffInputRowView[];

  isEmpty: boolean;

  hasDefinitionDivergence: boolean;

  liveSide: "base" | "against" | null;
  baseLabel: string;
  againstLabel: string;
  terminalBase: string;
  terminalAgainst: string;
}

function formatBytes(size: number): string {
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function formatInline(value: unknown): string {
  if (typeof value === "string") return value;
  if (value === undefined) return "";
  return JSON.stringify(value);
}

function projectValue(value: LoopDiffValue | undefined): LoopDiffValueView {
  if (!value) {
    return { text: "", summary: "", isSummarized: false, isAbsent: true };
  }
  if (value.inline !== undefined) {
    return {
      text: formatInline(value.inline),
      summary: "",
      isSummarized: false,
      isAbsent: false,
    };
  }
  const parts: string[] = [];
  if (value.size !== undefined) parts.push(formatBytes(value.size));
  if (value.hash) parts.push(value.hash);
  return {
    text: "",
    summary: parts.join(" · "),
    isSummarized: parts.length > 0,
    isAbsent: parts.length === 0,
  };
}

function projectRow(node: LoopDiffNode, index: number): LoopDiffRowView {
  const change: LoopDiffChange = isLoopDiffChange(node.change) ? node.change : "changed";
  return {
    key: `${node.node_id}:${node.item_index ?? "x"}:${index}`,
    nodeId: node.node_id,
    itemIndex: node.item_index ?? null,
    change,
    changeLabel: LOOP_DIFF_CHANGE_LABEL[change],
    tone: LOOP_DIFF_CHANGE_TONE[change],
    cause: node.cause ?? "",
    base: projectValue(node.base),
    against: projectValue(node.against),
  };
}

function projectInput(input: LoopDiffInput): LoopDiffInputRowView {
  const base = projectValue(input.base);
  const against = projectValue(input.against);
  return {
    key: input.key,
    base,
    against,
    changed: base.text !== against.text || base.summary !== against.summary,
  };
}

function sideLabel(side: LoopDiff["base"]): string {
  return `${side.run_id} · generation ${side.generation}`;
}

export function projectLoopDiff(diff: LoopDiff): LoopDiffView {
  const rows = diff.nodes.map(projectRow);

  const byChange = new Map<LoopDiffChange, LoopDiffRowView[]>();
  for (const row of rows) {
    const bucket = byChange.get(row.change);
    if (bucket) bucket.push(row);
    else byChange.set(row.change, [row]);
  }
  const groups: LoopDiffGroupView[] = [];
  for (const change of LOOP_DIFF_CHANGES) {
    const grouped = byChange.get(change);
    if (!grouped || grouped.length === 0) continue;
    groups.push({
      change,
      label: LOOP_DIFF_CHANGE_LABEL[change],
      tone: LOOP_DIFF_CHANGE_TONE[change],
      rows: grouped,
    });
  }
  const inputs = diff.inputs.map(projectInput);
  const baseLive = !isTerminalLoopStatus(diff.base.status);
  const againstLive = !isTerminalLoopStatus(diff.against.status);
  return {
    kind: diff.kind,
    isRunCompare: diff.kind === "run",
    groups,
    inputs,
    isEmpty: rows.length === 0 && inputs.every(input => !input.changed),
    hasDefinitionDivergence: diff.definition_divergence === true,
    liveSide: baseLive ? "base" : againstLive ? "against" : null,
    baseLabel: sideLabel(diff.base),
    againstLabel: sideLabel(diff.against),
    terminalBase: diff.terminal?.base ?? diff.base.status,
    terminalAgainst: diff.terminal?.against ?? diff.against.status,
  };
}

export function comparableGenerations(generations: readonly { generation: number }[]): number[] {
  return [...new Set(generations.map(entry => entry.generation))].sort((a, b) => b - a);
}
