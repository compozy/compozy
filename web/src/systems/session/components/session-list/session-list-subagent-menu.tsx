import { useRef, type HTMLAttributes, type ReactElement } from "react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuTrigger,
  MenuTreeItems,
  tidyMenuTree,
  type MenuTreeNode,
} from "@compozy/ui";
import { Clipboard, Copy, Hash, Square, Trash2 } from "@compozy/ui/icons";

import { openContextMenuFromKeyboard } from "@/systems/os/lib/context-menu-keyboard";

import { copySessionText } from "../../lib/session-copy";
import { isSubagentStoppable } from "../subagents/subagent-format";
import type { SubagentView } from "../subagents/types";

export interface SessionListSubagentMenuProps {
  subagent: SubagentView;
  /** The trigger's test id; items append their verb, e.g. `${menuTestId}-copy-id`. */
  menuTestId: string;
  /** The row element; it becomes the right-click and Shift+F10 target. */
  render: ReactElement<HTMLAttributes<HTMLElement>>;
  onStop?: (subagent: SubagentView) => void;
  onDelete?: (subagent: SubagentView) => void;
}

/**
 * Right-click (and Shift+F10) menu for one subagent row on the rail: the verbs a
 * delegated row can take. Copy is always offered; Stop only while it is live and
 * delegated; Delete only for a delegated row, behind its confirmation. A
 * provider-native row follows its parent turn, so it offers neither.
 */
export function SessionListSubagentMenu({
  subagent,
  menuTestId,
  render,
  onStop,
  onDelete,
}: SessionListSubagentMenuProps) {
  const trigger = useRef<HTMLDivElement>(null);
  const nodes = tidyMenuTree([
    {
      kind: "sub",
      id: "copy",
      label: "Copy",
      icon: Clipboard,
      testId: `${menuTestId}-copy`,
      children: [
        {
          kind: "item",
          id: "copy-id",
          label: "Copy subagent ID",
          icon: Hash,
          testId: `${menuTestId}-copy-id`,
          onSelect: () => void copySessionText(subagent.id, "Subagent ID"),
        },
        {
          kind: "item",
          id: "copy-title",
          label: "Copy title",
          icon: Copy,
          testId: `${menuTestId}-copy-title`,
          onSelect: () => void copySessionText(subagent.title, "Subagent title"),
        },
      ],
    },
    ...(onStop && isSubagentStoppable(subagent)
      ? [
          {
            kind: "item",
            id: "stop",
            label: "Stop subagent",
            icon: Square,
            testId: `${menuTestId}-stop`,
            onSelect: () => onStop(subagent),
          } as const,
        ]
      : []),
    ...(onDelete && subagent.origin !== "provider_native"
      ? [
          { kind: "separator", id: "delete-separator" } as const,
          {
            kind: "item",
            id: "delete",
            label: "Delete subagent",
            icon: Trash2,
            destructive: true,
            testId: `${menuTestId}-delete`,
            onSelect: () => onDelete(subagent),
          } as const,
        ]
      : []),
  ] satisfies MenuTreeNode[]);

  return (
    <ContextMenu>
      <ContextMenuTrigger
        ref={trigger}
        data-testid={menuTestId}
        render={render}
        onKeyDown={openContextMenuFromKeyboard}
      />
      <ContextMenuContent
        className="min-w-56"
        // Escape hands focus back to the row, so the rail's keys keep working.
        finalFocus={closeType =>
          closeType === "keyboard" ? subagentRowFocusTarget(trigger.current) : false
        }
      >
        <MenuTreeItems host="context" nodes={nodes} />
      </ContextMenuContent>
    </ContextMenu>
  );
}

/** The row's own focus target: the trigger when it takes focus, else its first button. */
function subagentRowFocusTarget(trigger: HTMLElement | null): HTMLElement | null {
  if (!trigger || trigger.tabIndex >= 0) return trigger;
  return trigger.querySelector<HTMLElement>("button");
}
