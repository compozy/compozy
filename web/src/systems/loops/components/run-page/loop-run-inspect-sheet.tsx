import { CodeXml, Search, Workflow } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  buttonVariants,
  Eyebrow,
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@agh/ui";

import type { LoopGateVerdict } from "../../lib/loop-events";
import { type LoopGraph, fanOutSummary, findWatchNode } from "../../lib/loop-graph";
import { onExceededPolicy } from "../../lib/loop-limits";
import type {
  LoopDefinition,
  LoopRunEventFrame,
  LoopRunRecord,
  LoopWatchEventsState,
} from "../../types";
import {
  LoopRunInspectCriteria,
  LoopRunInspectEvents,
  LoopRunInspectTerminalStates,
  LoopRunInspectTiles,
  LoopRunInspectVerification,
  LoopRunInspectWatch,
  type LoopRunInspectTile,
} from "./loop-run-inspect-sections";

interface LoopRunInspectSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  run: LoopRunRecord;
  definition?: LoopDefinition;
  graph: LoopGraph | null;
  latestVerdict: LoopGateVerdict | null;
  watchEvents?: LoopWatchEventsState;
  frames: readonly LoopRunEventFrame[];
}

function str(value: unknown): string {
  return typeof value === "string" ? value : "";
}

/** The watch tile value (`poll 30s · settle 20s` / `events × 2`). */
function watchTileValue(graph: LoopGraph | null): string {
  const node = graph ? findWatchNode(graph) : null;
  if (!node) return "—";
  const parts: string[] = [];
  const poll = str(node.watch?.poll);
  const settle = str(node.watch?.settle);
  if (poll) parts.push(`poll ${poll}`);
  if (settle) parts.push(`settle ${settle}`);
  if (node.eventsCount > 0) parts.push(`events × ${node.eventsCount}`);
  return parts.length > 0 ? parts.join(" · ") : node.id;
}

function fanOutTileValue(graph: LoopGraph | null): string {
  const node = graph?.nodes.find(candidate => candidate.kind === "fan-out");
  if (!node) return "—";
  return fanOutSummary(node) ?? node.id;
}

/**
 * Single-line `budget_on_exceeded` label for the inspect "On limit" tile. Mirrors
 * the split `value`/`ceiling` copy `buildLoopLimits` renders on the limits rail
 * (`lib/loop-limits.ts`) — the two are the same policy→outcome vocabulary.
 */
function onExceededPolicyLabel(policy: LoopRunRecord["budget_on_exceeded"]): string {
  const { policy: label, target } = onExceededPolicy(policy);
  return `${label} ${target}`;
}

function buildTiles(
  run: LoopRunRecord,
  definition: LoopDefinition | undefined,
  graph: LoopGraph | null
): LoopRunInspectTile[] {
  const contract = definition?.contract;
  const verification = contract?.verification ?? [];
  const verificationValue =
    verification.length === 0
      ? "—"
      : verification.length === 1
        ? `${verification[0].id} · ${verification[0].type}`
        : `${verification[0].id} · ${verification[0].type} +${verification.length - 1}`;
  return [
    { label: "Stop when", value: contract?.stop_when || "—" },
    { label: "Verification", value: verificationValue },
    { label: "Re-attempt", value: run.reattempt_strategy },
    { label: "Concurrency", value: definition?.concurrency || "—" },
    { label: "Watch source", value: watchTileValue(graph) },
    { label: "Fan-out", value: fanOutTileValue(graph) },
    { label: "On limit", value: onExceededPolicyLabel(run.budget_on_exceeded) },
    {
      label: "No-progress window",
      value:
        contract?.no_progress.window !== undefined
          ? `${contract.no_progress.window} generations`
          : "—",
    },
  ];
}

/**
 * The Inspect drawer (§2/§4): the operator sheet holding everything the story
 * surface dropped — stop_when, verification, policies, watch spec, fan-out
 * params, the last check's criteria, watch cursors, and the raw event frames.
 */
export function LoopRunInspectSheet({
  open,
  onOpenChange,
  run,
  definition,
  graph,
  latestVerdict,
  watchEvents,
  frames,
}: LoopRunInspectSheetProps) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        className="gap-0 sm:max-w-[560px]"
        data-testid="loop-run-inspect-sheet"
        side="right"
      >
        <SheetHeader className="flex-none flex-row items-start gap-3 border-b border-line p-5">
          <span
            aria-hidden="true"
            className="grid size-9 flex-none place-items-center rounded-md bg-accent-tint text-accent-strong"
          >
            <Search className="size-4" />
          </span>
          <div className="flex min-w-0 flex-col gap-0.5">
            <Eyebrow className="text-subtle">Operator</Eyebrow>
            <SheetTitle className="text-item-title font-medium text-fg-strong">
              Inspect run
            </SheetTitle>
            <SheetDescription className="text-small-body text-muted">
              Loop mechanics for{" "}
              <span className="font-mono text-mono-id tabular-nums">{run.id}</span>
              {run.definition_digest ? (
                <>
                  {" · digest "}
                  <span className="font-mono text-mono-id tabular-nums">
                    {run.definition_digest}
                  </span>
                </>
              ) : null}
            </SheetDescription>
          </div>
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-5">
          <div className="flex gap-2">
            <Link
              className={buttonVariants({ variant: "outline", size: "sm" })}
              params={{ name: run.loop_name }}
              to="/loops/$name/editor"
            >
              <Workflow aria-hidden="true" className="size-3.5" />
              View graph
            </Link>
            <Link
              className={buttonVariants({ variant: "outline", size: "sm" })}
              params={{ name: run.loop_name }}
              to="/loops/$name"
            >
              <CodeXml aria-hidden="true" className="size-3.5" />
              View definition
            </Link>
          </div>
          <LoopRunInspectTiles tiles={buildTiles(run, definition, graph)} />
          <LoopRunInspectVerification verification={definition?.contract?.verification ?? []} />
          <LoopRunInspectTerminalStates states={definition?.contract?.terminal_states ?? []} />
          <LoopRunInspectCriteria verdict={latestVerdict} />
          <LoopRunInspectWatch state={watchEvents} />
          <LoopRunInspectEvents frames={frames} />
        </div>
      </SheetContent>
    </Sheet>
  );
}
