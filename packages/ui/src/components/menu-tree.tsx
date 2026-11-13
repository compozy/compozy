/**
 * One menu tree, two hosts. A product builds its menu once as `MenuTreeNode`s; `MenuTreeItems`
 * renders the same tree inside a context menu (right-click) or a dropdown (`…`), so the two
 * entry points of one object can never drift. Grouping follows DESIGN.md: frequent verbs at the
 * top level, related verbs behind a submenu, destructive verbs last.
 */
import { Check } from "../icons";
import type { MenuTreeHost, MenuTreeNode } from "../lib/menu-tree";

import {
  ContextMenuGroup,
  ContextMenuItem,
  ContextMenuLabel,
  ContextMenuSeparator,
  ContextMenuShortcut,
  ContextMenuSub,
  ContextMenuSubContent,
  ContextMenuSubTrigger,
} from "./context-menu";
import {
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
  DropdownMenuSub,
  DropdownMenuSubContent,
  DropdownMenuSubTrigger,
} from "./dropdown-menu";

const KITS = {
  context: {
    Group: ContextMenuGroup,
    Item: ContextMenuItem,
    Label: ContextMenuLabel,
    Separator: ContextMenuSeparator,
    Shortcut: ContextMenuShortcut,
    Sub: ContextMenuSub,
    SubTrigger: ContextMenuSubTrigger,
    SubContent: ContextMenuSubContent,
  },
  dropdown: {
    Group: DropdownMenuGroup,
    Item: DropdownMenuItem,
    Label: DropdownMenuLabel,
    Separator: DropdownMenuSeparator,
    Shortcut: DropdownMenuShortcut,
    Sub: DropdownMenuSub,
    SubTrigger: DropdownMenuSubTrigger,
    SubContent: DropdownMenuSubContent,
  },
} as const;

/**
 * The tree's rows for `host`. Place it inside that host's content (`ContextMenuContent` or
 * `DropdownMenuContent`). `openSubs` names submenus that start expanded, for reviews and stories.
 */
export function MenuTreeItems({
  nodes,
  host,
  openSubs,
}: {
  nodes: readonly MenuTreeNode[];
  host: MenuTreeHost;
  openSubs?: ReadonlySet<string>;
}) {
  const { Group, Item, Label, Separator, Shortcut, Sub, SubTrigger, SubContent } = KITS[host];
  return (
    <>
      {nodes.map(node => {
        switch (node.kind) {
          case "separator":
            return <Separator key={node.id} />;
          case "group":
            return (
              <Group key={node.id}>
                <Label>{node.label}</Label>
                <MenuTreeItems nodes={node.children} host={host} openSubs={openSubs} />
              </Group>
            );
          case "sub":
            return (
              <Sub key={node.id} defaultOpen={openSubs?.has(node.id) ?? false}>
                <SubTrigger data-testid={node.testId} disabled={node.disabled}>
                  {node.icon ? <node.icon aria-hidden="true" className="size-3" /> : null}
                  <span className="min-w-0 truncate">{node.label}</span>
                  {node.hint ? (
                    <Shortcut className="mr-1 whitespace-nowrap">{node.hint}</Shortcut>
                  ) : null}
                </SubTrigger>
                <SubContent className="max-w-md min-w-52">
                  <MenuTreeItems nodes={node.children} host={host} openSubs={openSubs} />
                </SubContent>
              </Sub>
            );
          case "item": {
            const Icon = node.checked ? Check : node.icon;
            return (
              <Item
                key={node.id}
                closeOnClick={node.closeOnClick}
                data-testid={node.testId}
                disabled={node.disabled}
                onClick={node.onSelect}
                title={node.title}
                variant={node.destructive ? "destructive" : undefined}
              >
                {Icon ? <Icon aria-hidden="true" className="size-3" /> : null}
                <span className="min-w-0 truncate">{node.label}</span>
                {node.hint ? <Shortcut className="whitespace-nowrap">{node.hint}</Shortcut> : null}
              </Item>
            );
          }
        }
      })}
    </>
  );
}
