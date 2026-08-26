import type { LucideIcon } from "lucide-react";
import {
  Bot,
  CheckCheck,
  Eye,
  MessageCircleQuestion,
  FileDown,
  GitBranch,
  GitMerge,
  Hourglass,
  LogIn,
  Parentheses,
  Radio,
  Repeat2,
  RotateCcw,
  Send,
  Signpost,
  Split,
  Target,
  Wrench,
} from "lucide-react";

import type { LoopNodeClass } from "./loop-graph";

/**
 * Closed DSL kinds from the editor palette (`loop-palette.ts`).
 * Open ToolID strings are not in this map — use the class-level glyph.
 */
export const LOOP_NODE_KIND_ICONS = {
  "run-agent": Bot,
  goal: Target,
  "run-loop": RotateCcw,
  transform: Parentheses,
  compozy__network_send: Send,
  "fan-out": Split,
  collect: GitMerge,
  branch: GitBranch,
  gate: CheckCheck,
  "sub-loop": Repeat2,
  wait: Hourglass,
  ask: MessageCircleQuestion,
  route: Signpost,
  "watch-source": Eye,
  "watch-events": Radio,
  "file-import": FileDown,
  input: LogIn,
} as const satisfies Record<string, LucideIcon>;

export type LoopNodeKind = keyof typeof LOOP_NODE_KIND_ICONS;

/** Blank ToolID action in the palette (`Call tool…`). */
export const LOOP_CALL_TOOL_ICON: LucideIcon = Wrench;

/**
 * Detail-DAG class glyphs. One icon per concept — never a per-ToolID mark.
 * `control+fan-out` and `control+gate` win over plain control.
 */
export const LOOP_NODE_CLASS_ICONS = {
  source: LogIn,
  "control-fan-out": Split,
  "control-gate": CheckCheck,
  "control-route": Signpost,
  "control-ask": MessageCircleQuestion,
  control: GitMerge,
  action: Bot,
} as const satisfies Record<string, LucideIcon>;

export function loopNodeClassIcon(input: {
  nodeClass: LoopNodeClass;
  isFanOut?: boolean;
  isGate?: boolean;
  isRoute?: boolean;
  isAsk?: boolean;
}): LucideIcon {
  if (input.nodeClass === "source") return LOOP_NODE_CLASS_ICONS.source;
  if (input.nodeClass === "action") return LOOP_NODE_CLASS_ICONS.action;
  if (input.isFanOut) return LOOP_NODE_CLASS_ICONS["control-fan-out"];
  if (input.isGate) return LOOP_NODE_CLASS_ICONS["control-gate"];
  if (input.isRoute) return LOOP_NODE_CLASS_ICONS["control-route"];
  if (input.isAsk) return LOOP_NODE_CLASS_ICONS["control-ask"];
  return LOOP_NODE_CLASS_ICONS.control;
}
