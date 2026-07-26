import { CodeXml, Ellipsis, Search, Workflow } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@agh/ui";

interface LoopRunOverflowMenuProps {
  loopName: string;
  onInspect: () => void;
}

/**
 * The topbar ⋯ overflow (§3): the operator views that left the surface — View
 * graph (loop editor), View definition (loop detail), and the Inspect drawer.
 * Renders for every status, including terminal runs whose controls are gone.
 */
export function LoopRunOverflowMenu({ loopName, onInspect }: LoopRunOverflowMenuProps) {
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
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
