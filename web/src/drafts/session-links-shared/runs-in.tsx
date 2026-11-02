/** Where the agent's process runs, kept apart from everything merely linked. */
import { FolderGit2, FolderIcon } from "@compozy/ui/icons";

import { Icon, Item, ItemContent, ItemDescription, ItemMedia } from "@compozy/ui";

import type { SessionRunsIn } from "@/systems/session/mocks/session-links-fixtures";

export function RunsIn({ place }: { place: SessionRunsIn }) {
  return (
    <div className="flex flex-col gap-1.5" data-slot="session-runs-in">
      <Item size="xs" variant="muted">
        <ItemMedia variant="icon">
          <Icon as={place.kind === "worktree" ? FolderGit2 : FolderIcon} className="text-subtle" />
        </ItemMedia>
        <ItemContent className="min-w-0">
          <span className="truncate text-small-body text-muted-foreground">
            This session runs in <span className="font-medium text-fg">{place.name}</span>
          </span>
        </ItemContent>
      </Item>
      <ItemDescription className="px-2.5 text-eyebrow">
        Linked work is a shortcut. It never changes where the agent runs.
      </ItemDescription>
    </div>
  );
}
