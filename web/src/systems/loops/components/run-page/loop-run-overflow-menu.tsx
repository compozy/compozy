import { CodeXml, Ellipsis, Search, Workflow, Zap } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@compozy/ui";

interface LoopRunOverflowMenuProps {
  loopName: string;
  onInspect: () => void;
  /**
   * Kill lives here, not beside Cancel: it is the destructive escape, and the
   * surface keeps exactly one ⋯ menu (DESIGN-LESSONS L12). Omitted entirely for
   * a terminal run, where the daemon would reject it.
   */
  onKill?: () => void;
  isKillPending?: boolean;
}

/**
 * The topbar ⋯ overflow (§3): the operator views that left the surface — View
 * graph (loop editor), View definition (loop detail), and the Inspect drawer —
 * plus Kill for a live run.
 * Renders for every status, including terminal runs whose controls are gone.
 */
export function LoopRunOverflowMenu({
  loopName,
  onInspect,
  onKill,
  isKillPending,
}: LoopRunOverflowMenuProps) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More"
        data-testid="loop-run-more"
        render={<Button type="button" variant="ghost" size="icon-sm" />}
      >
        <Ellipsis aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-49">
        <DropdownMenuItem
          data-testid="loop-run-view-graph"
          render={<Link params={{ name: loopName }} to="/loops/$name/editor" />}
        >
          <Workflow aria-hidden="true" className="size-3.5" />
          View graph
        </DropdownMenuItem>
        <DropdownMenuItem
          data-testid="loop-run-view-definition"
          render={<Link params={{ name: loopName }} to="/loops/$name" />}
        >
          <CodeXml aria-hidden="true" className="size-3.5" />
          View definition
        </DropdownMenuItem>
        <DropdownMenuItem data-testid="loop-run-inspect" onClick={onInspect}>
          <Search aria-hidden="true" className="size-3.5" />
          Inspect run
        </DropdownMenuItem>
        {onKill ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="text-danger focus:text-danger"
              data-testid="loop-run-kill"
              disabled={isKillPending}
              onClick={onKill}
            >
              <Zap aria-hidden="true" className="size-3.5" />
              Kill run…
            </DropdownMenuItem>
          </>
        ) : null}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
