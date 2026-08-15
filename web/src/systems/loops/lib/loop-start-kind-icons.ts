import type { LucideIcon } from "lucide-react";
import {
  CalendarClock,
  Globe,
  Hand,
  Plug,
  Puzzle,
  RadioTower,
  Terminal,
  Webhook,
  Wrench,
  Zap,
} from "lucide-react";

/**
 * Closed `start[]` vocabulary from `loop-bindings.ts`:
 * automation-carried `schedule|webhook|trigger` plus the direct surfaces
 * `manual|cli|http|uds|native_tool|extension|network`.
 */
export const LOOP_START_KINDS = [
  "schedule",
  "cli",
  "manual",
  "http",
  "uds",
  "native_tool",
  "webhook",
  "trigger",
  "extension",
  "network",
] as const;

export type LoopStartKind = (typeof LOOP_START_KINDS)[number];

const LOOP_START_KIND_SET = new Set<string>(LOOP_START_KINDS);

export const LOOP_START_KIND_ICONS: Record<LoopStartKind, LucideIcon> = {
  schedule: CalendarClock,
  cli: Terminal,
  manual: Hand,
  http: Globe,
  uds: Plug,
  native_tool: Wrench,
  webhook: Webhook,
  trigger: Zap,
  extension: Puzzle,
  network: RadioTower,
};

export function loopStartKindIcon(kind: string): LucideIcon | undefined {
  if (!LOOP_START_KIND_SET.has(kind)) return undefined;
  return LOOP_START_KIND_ICONS[kind as LoopStartKind];
}
