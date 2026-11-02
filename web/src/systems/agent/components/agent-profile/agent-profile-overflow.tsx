import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

/** Secondary agent actions in the window-head overflow. */
export function AgentProfileOverflow({
  onDuplicate,
  onDelete,
}: {
  onDuplicate: () => void;
  onDelete: () => void;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="More agent actions"
        data-testid="agent-page-overflow"
        render={<Button size="icon-sm" type="button" variant="quiet" />}
      >
        <TopbarOverflowIcon aria-hidden="true" className="size-4" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="agent-page-overflow-menu">
        <DropdownMenuItem data-testid="agent-page-duplicate" onClick={onDuplicate}>
          Duplicate
        </DropdownMenuItem>
        <DropdownMenuItem data-testid="agent-page-delete" onClick={onDelete} variant="destructive">
          Delete…
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
