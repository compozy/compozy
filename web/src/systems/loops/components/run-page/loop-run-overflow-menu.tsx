import { CodeXml, Ellipsis, Workflow } from "lucide-react";
import { Link } from "@tanstack/react-router";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";

interface LoopRunOverflowMenuProps {
  loopName: string;
}

export function LoopRunOverflowMenu({ loopName }: LoopRunOverflowMenuProps) {
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
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
