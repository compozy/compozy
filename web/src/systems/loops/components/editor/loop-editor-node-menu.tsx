import type { ReactNode } from "react";
import { ClipboardPaste, Copy, CopyPlus, Pencil, Trash2 } from "lucide-react";

import {
  ContextMenu,
  ContextMenuContent,
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuTrigger,
  Kbd,
} from "@compozy/ui";

import { openContextMenuFromKeyboard } from "@/systems/os/lib/context-menu-keyboard";

export interface LoopEditorNodeMenuProps {
  nodeId: string;
  readOnly?: boolean;
  canPaste?: boolean;
  children: ReactNode;
  onDuplicate: (nodeId: string) => void;
  onCopy: (nodeId: string) => void;
  onPaste: () => void;
  onRename: (nodeId: string) => void;
  onDelete: (nodeId: string) => void;
}

export function LoopEditorNodeMenu({
  nodeId,
  readOnly = false,
  canPaste = false,
  children,
  onDuplicate,
  onCopy,
  onPaste,
  onRename,
  onDelete,
}: LoopEditorNodeMenuProps) {
  return (
    <ContextMenu>
      <ContextMenuTrigger
        data-testid={`loop-editor-node-menu-${nodeId}`}
        render={
          <div role="presentation" onKeyDown={openContextMenuFromKeyboard}>
            {children}
          </div>
        }
      />
      <ContextMenuContent className="min-w-44">
        {readOnly ? (
          <ContextMenuGroup>
            <ContextMenuLabel>Read-only definition</ContextMenuLabel>
          </ContextMenuGroup>
        ) : (
          <>
            <ContextMenuItem
              data-testid="loop-node-menu-duplicate"
              onClick={() => onDuplicate(nodeId)}
            >
              <CopyPlus aria-hidden="true" />
              Duplicate
            </ContextMenuItem>
            <ContextMenuItem data-testid="loop-node-menu-copy" onClick={() => onCopy(nodeId)}>
              <Copy aria-hidden="true" />
              Copy
            </ContextMenuItem>
            <ContextMenuItem
              data-testid="loop-node-menu-paste"
              disabled={!canPaste}
              onClick={onPaste}
            >
              <ClipboardPaste aria-hidden="true" />
              {canPaste ? "Paste" : "Paste (nothing copied)"}
            </ContextMenuItem>
            <ContextMenuItem data-testid="loop-node-menu-rename" onClick={() => onRename(nodeId)}>
              <Pencil aria-hidden="true" />
              Rename
            </ContextMenuItem>
            <ContextMenuSeparator />
            <ContextMenuItem
              data-testid="loop-node-menu-delete"
              onClick={() => onDelete(nodeId)}
              variant="destructive"
            >
              <Trash2 aria-hidden="true" />
              Delete
              <ContextMenuShortcut>
                <Kbd>⌫</Kbd>
              </ContextMenuShortcut>
            </ContextMenuItem>
          </>
        )}
      </ContextMenuContent>
    </ContextMenu>
  );
}
