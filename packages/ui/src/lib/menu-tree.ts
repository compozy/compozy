/** The menu tree model shared by `MenuTreeItems`: nodes and the tidy pass hosts run before rendering. */
import type { LucideIcon } from "../icons";

interface MenuTreeEntry {
  /** Stable within its parent; React key and default submenu id. */
  id: string;
  label: string;
  icon?: LucideIcon;
  /** Right-aligned secondary text: a count, a status, a keyboard shortcut. */
  hint?: string;
  testId?: string;
}

export type MenuTreeNode =
  | (MenuTreeEntry & {
      kind: "item";
      destructive?: boolean;
      disabled?: boolean;
      /** The current choice of a set (a folder, a profile): drawn with a check. */
      checked?: boolean;
      /** Native tooltip: why the item is unavailable, or what it will do. */
      title?: string;
      /** False keeps the menu open on click, for an informational row such as a blocked reason. */
      closeOnClick?: boolean;
      onSelect?: () => void;
    })
  | (MenuTreeEntry & { kind: "sub"; disabled?: boolean; children: readonly MenuTreeNode[] })
  | { kind: "group"; id: string; label: string; children: readonly MenuTreeNode[] }
  | { kind: "separator"; id: string };

export type MenuTreeHost = "context" | "dropdown";

/**
 * Drops what a conditional tree leaves behind: empty submenus and groups, and separators that
 * lead, trail, or follow another separator.
 */
export function tidyMenuTree(nodes: readonly MenuTreeNode[]): MenuTreeNode[] {
  const out: MenuTreeNode[] = [];
  for (const node of nodes) {
    if (node.kind === "sub" || node.kind === "group") {
      const children = tidyMenuTree(node.children);
      if (children.length > 0) out.push({ ...node, children });
      continue;
    }
    if (node.kind === "separator" && (out.length === 0 || out.at(-1)?.kind === "separator")) {
      continue;
    }
    out.push(node);
  }
  while (out.at(-1)?.kind === "separator") out.pop();
  return out;
}
