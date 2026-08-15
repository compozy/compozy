import { CodeXml, Ellipsis, Workflow, Zap } from "lucide-react";
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
  /**
   * Kill lives here, not beside Cancel: it is the destructive escape, and the
   * surface keeps exactly one ⋯ menu (DESIGN-LESSONS L12). Omitted entirely for
   * a terminal run, where the daemon would reject it.
   */
  onKill?: () => void;
  isKillPending?: boolean;
}

/**
 * The topbar ⋯ overflow (§3): View graph and View definition, plus Kill for a
 * live run. Inspect lives once, in the rail foot beside the digest.
 * Renders for every status, including terminal runs whose controls are gone.
 */
export function LoopRunOverflowMenu({ loopName, onKill, isKillPending }: LoopRunOverflowMenuProps) {
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
