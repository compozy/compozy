import { Play } from "lucide-react";

import { Button } from "@compozy/ui";

import { type LoopNodeVerb, loopNodeVerbs } from "../../lib/loop-node-controls";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { LoopNodeControlMenu } from "./loop-node-control-menu";

interface LoopNodeRowActionsProps {
  node: LoopNodeLifecycle;
  runStatus?: string | null;
  isPending?: boolean;
  onVerb: (verb: LoopNodeVerb, node: LoopNodeLifecycle) => void;
}

/**
 * The trailing action cluster on a lifecycle row.
 *
 * A parked lane gets its expected verb as a first-class button — Resume for a
 * paused node, Requeue for a quarantined one — because that is the action the
 * operator came to the row to take. Everything else stays in the `⋯` menu, and
 * the promoted verb is only ever one `loopNodeVerbs` already allows, so this
 * shortcut can never outrun daemon truth.
 */
export function LoopNodeRowActions({
  node,
  runStatus,
  isPending,
  onVerb,
}: LoopNodeRowActionsProps) {
  const verbs = loopNodeVerbs(node, runStatus);
  const primary: LoopNodeVerb | null = verbs.includes("resume")
    ? "resume"
    : verbs.includes("requeue")
      ? "requeue"
      : null;
  return (
    <span className="flex shrink-0 items-center gap-1.5">
      {primary ? (
        <Button
          className="min-h-6"
          data-testid={`loop-node-primary-${primary}-${node.nodeId}`}
          disabled={isPending}
          onClick={() => onVerb(primary, node)}
          size="sm"
          type="button"
          variant="outline"
        >
          <Play aria-hidden="true" className="size-3" />
          {primary === "resume" ? "Resume" : "Requeue…"}
        </Button>
      ) : null}
      <LoopNodeControlMenu
        isPending={isPending}
        node={node}
        onVerb={onVerb}
        runStatus={runStatus}
      />
    </span>
  );
}
