import { MoreHorizontal } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuTrigger,
} from "@compozy/ui";

import {
  LOOP_NODE_VERB_PRESENTATION,
  type LoopNodeVerb,
  loopNodeVerbs,
} from "../../lib/loop-node-controls";
import { LOOP_NODE_VERB_ICONS } from "../../lib/loop-node-verb-icons";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";

interface LoopNodeControlMenuProps {
  node: LoopNodeLifecycle;
  runStatus?: string | null;
  isPending?: boolean;
  onVerb: (verb: LoopNodeVerb, node: LoopNodeLifecycle) => void;
}

/**
 * Verbs that end work sit below a separator so they are never the reflex click.
 * Danger styling remains owned by `LOOP_NODE_VERB_PRESENTATION`.
 */
const STOPPING_VERBS = new Set<LoopNodeVerb>(["cancel", "kill"]);

/**
 * The per-node verb menu (VC-R3). The item set is whatever
 * `loopNodeVerbs` derives from the node's own payload — this component adds no
 * verb of its own, so a state the daemon has not declared can never grow a
 * control here. An empty verb set renders no trigger at all rather than a
 * disabled button that implies an action exists.
 */
export function LoopNodeControlMenu({
  node,
  runStatus,
  isPending,
  onVerb,
}: LoopNodeControlMenuProps) {
  const verbs = loopNodeVerbs(node, runStatus);
  if (verbs.length === 0) return null;
  const safeVerbs = verbs.filter(verb => !STOPPING_VERBS.has(verb));
  const stoppingVerbs = verbs.filter(verb => STOPPING_VERBS.has(verb));
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            aria-label={`Actions for ${node.label}`}
            className="min-h-6 shrink-0"
            data-testid={`loop-node-menu-trigger-${node.nodeId}`}
            disabled={isPending}
            size="sm"
            type="button"
            variant="ghost"
          />
        }
      >
        <MoreHorizontal aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        {safeVerbs.map(verb => {
          const presentation = LOOP_NODE_VERB_PRESENTATION[verb];
          const Icon = LOOP_NODE_VERB_ICONS[verb];
          return (
            <DropdownMenuItem
              data-testid={`loop-node-verb-${verb}`}
              key={verb}
              onClick={() => onVerb(verb, node)}
            >
              <Icon aria-hidden="true" className="size-3.5" />
              {presentation.label}
              {presentation.mode ? (
                <DropdownMenuShortcut>{presentation.mode}</DropdownMenuShortcut>
              ) : null}
            </DropdownMenuItem>
          );
        })}
        {safeVerbs.length > 0 && stoppingVerbs.length > 0 ? <DropdownMenuSeparator /> : null}
        {stoppingVerbs.map(verb => {
          const presentation = LOOP_NODE_VERB_PRESENTATION[verb];
          const Icon = LOOP_NODE_VERB_ICONS[verb];
          return (
            <DropdownMenuItem
              className={presentation.destructive ? "text-danger focus:text-danger" : undefined}
              data-testid={`loop-node-verb-${verb}`}
              key={verb}
              onClick={() => onVerb(verb, node)}
            >
              <Icon aria-hidden="true" className="size-3.5" />
              {presentation.label}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
