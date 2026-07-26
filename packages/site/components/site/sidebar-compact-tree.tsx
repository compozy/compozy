"use client";

import { cn } from "@agh/ui";
import type { Folder, Item } from "fumadocs-core/page-tree";
import {
  SidebarFolder,
  SidebarFolderContent,
  SidebarFolderLink,
  SidebarFolderTrigger,
  SidebarItem,
  useFolderDepth,
} from "fumadocs-ui/components/sidebar/base";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

function normalize(href: string): string {
  return href.length > 1 && href.endsWith("/") ? href.slice(0, -1) : href;
}

function isActive(href: string, pathname: string): boolean {
  return normalize(href) === normalize(pathname);
}

const ITEM_CLS =
  "relative flex flex-row items-center gap-1.5 rounded-md px-2 py-1 text-small-body text-fd-muted-foreground transition-colors hover:bg-fd-accent/40 hover:text-fd-accent-foreground data-[active=true]:bg-fd-primary/10 data-[active=true]:text-fd-primary [&_svg]:size-3 [&_svg]:shrink-0";

function offset(depth: number) {
  return `calc(0.5rem + 0.75rem * ${depth})`;
}

export function CompactItem({ item }: { item: Item }) {
  const pathname = usePathname();
  const depth = useFolderDepth();
  return (
    <SidebarItem
      href={item.url}
      external={item.external}
      active={isActive(item.url, pathname)}
      icon={item.icon}
      className={ITEM_CLS}
      style={{ paddingInlineStart: offset(depth) }}
    >
      {item.name}
    </SidebarItem>
  );
}

export function CompactFolder({ item, children }: { item: Folder; children: ReactNode }) {
  const pathname = usePathname();
  const depth = useFolderDepth();
  const headerCls = cn(ITEM_CLS, "w-full");
  const headerStyle = { paddingInlineStart: offset(depth) };
  return (
    <SidebarFolder collapsible={item.collapsible} defaultOpen={item.defaultOpen}>
      {item.index ? (
        <SidebarFolderLink
          href={item.index.url}
          active={isActive(item.index.url, pathname)}
          external={item.index.external}
          className={headerCls}
          style={headerStyle}
        >
          {item.icon}
          {item.name}
        </SidebarFolderLink>
      ) : (
        <SidebarFolderTrigger className={headerCls} style={headerStyle}>
          {item.icon}
          {item.name}
        </SidebarFolderTrigger>
      )}
      <SidebarFolderContent>{children}</SidebarFolderContent>
    </SidebarFolder>
  );
}
