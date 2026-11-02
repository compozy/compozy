import { FolderPlus, MessageSquarePlus, Plus, UserPlus } from "@compozy/ui/icons";
import type { ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Icon,
  cn,
} from "@compozy/ui";

export interface SessionListCreateMenuProps {
  onNewSession: () => void;
  /** Present where folders apply; turns (+) into a New session / New folder menu. */
  onNewFolder?: () => void;
  /** Why folders cannot be added here; shows New folder disabled with the reason. */
  newFolderUnavailable?: string | null;
  /** Present where a list shows one concrete scope: adds the Add stays-on agent item. */
  onAddStaysOn?: () => void;
  /** Rendered over the `+`: the picker that item opens. */
  overlay?: ReactNode;
  /** Accessible name of the single-action button and of the menu trigger. */
  label?: { session: string; menu: string };
  variant?: "secondary" | "quiet";
  size?: "icon-sm" | "icon-xs";
  className?: string;
  /** `<id>-new-session` / `<id>-new` / `<id>-new-folder`. */
  testId: string;
}

const DEFAULT_LABEL = { session: "New session", menu: "New session or folder" };

/**
 * A session list's create control. Secondary, not the inverted primary: the
 * composer's send owns that (P4). Where folders apply — or where they do not and
 * the operator should learn why — it opens a menu; elsewhere it creates a
 * session directly.
 */
export function SessionListCreateMenu({
  onNewSession,
  onNewFolder,
  newFolderUnavailable,
  onAddStaysOn,
  overlay,
  label = DEFAULT_LABEL,
  variant = "secondary",
  size = "icon-sm",
  className,
  testId,
}: SessionListCreateMenuProps) {
  if (!onNewFolder && !newFolderUnavailable && !onAddStaysOn) {
    return (
      <Button
        type="button"
        variant={variant}
        size={size}
        className={cn("shrink-0", className)}
        aria-label={label.session}
        data-testid={`${testId}-new-session`}
        onClick={onNewSession}
      >
        <Icon as={Plus} size="sm" />
      </Button>
    );
  }
  return (
    <span className="relative inline-flex shrink-0">
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={label.menu}
          data-testid={`${testId}-new`}
          render={
            <Button
              type="button"
              variant={variant}
              size={size}
              className={cn("shrink-0", className)}
            />
          }
        >
          <Icon as={Plus} size="sm" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-52">
          <DropdownMenuItem data-testid={`${testId}-new-session`} onClick={() => onNewSession()}>
            <Icon as={MessageSquarePlus} size="sm" />
            New session
          </DropdownMenuItem>
          {onNewFolder || newFolderUnavailable ? (
            <DropdownMenuItem
              data-testid={`${testId}-new-folder`}
              disabled={!onNewFolder}
              onClick={onNewFolder}
            >
              <Icon as={FolderPlus} size="sm" />
              {onNewFolder ? (
                "New folder"
              ) : (
                <span className="flex min-w-0 flex-col">
                  New folder
                  <span
                    className="text-micro text-subtle"
                    data-testid={`${testId}-new-folder-unavailable`}
                  >
                    {newFolderUnavailable}
                  </span>
                </span>
              )}
            </DropdownMenuItem>
          ) : null}
          {onAddStaysOn ? (
            <DropdownMenuItem data-testid={`${testId}-add-stays-on`} onClick={onAddStaysOn}>
              <Icon as={UserPlus} size="sm" />
              <span className="flex flex-col">
                Add stays-on agent
                <span className="text-eyebrow text-subtle">
                  Stays here and checks in on its own
                </span>
              </span>
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      {overlay}
    </span>
  );
}
