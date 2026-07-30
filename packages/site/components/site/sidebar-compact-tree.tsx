"use client";

import { cn } from "@compozy/ui";
import type { Folder, Item } from "fumadocs-core/page-tree";
import {
  SidebarFolder,
  SidebarFolderContent,
  SidebarFolderLink,
  SidebarFolderTrigger,
  SidebarItem,
  useFolderDepth,
} from "fumadocs-ui/components/sidebar/base";
import { useTreePath } from "fumadocs-ui/contexts/tree";
import { usePathname } from "next/navigation";
import type { ReactNode } from "react";
import { FolderOpenPersistWriter } from "./sidebar-folder-persist-effect";
import { folderPersistKey } from "./sidebar-folder-persist";

function normalize(href: string): string {
  return href.length > 1 && href.endsWith("/") ? href.slice(0, -1) : href;
}

function isActive(href: string, pathname: string): boolean {
  return normalize(href) === normalize(pathname);
}

function hasHoistedOverview(item: Folder): boolean {
  return (
    item.index !== undefined &&
    item.children.some(child => child.type === "page" && child.url === item.index?.url)
  );
}

const ITEM_CLS =
  "docs-sb-item relative flex h-(--height-sidebar-row) flex-row items-center gap-2 overflow-hidden rounded px-2 text-small-body font-normal text-muted transition-[color,background-color] duration-base ease-out hover:bg-elevated hover:text-fg data-[active=true]:bg-accent-tint data-[active=true]:font-medium data-[active=true]:text-accent [&_svg:not([data-icon])]:size-(--text-small-body) [&_svg:not([data-icon])]:shrink-0 [&_svg:not([data-icon])]:opacity-75 data-[active=true]:[&_svg:not([data-icon])]:opacity-100 [&_svg[data-icon]]:ms-auto [&_svg[data-icon]]:size-3 [&_svg[data-icon]]:shrink-0 [&_svg[data-icon]]:opacity-55";

function SidebarLabel({ name }: { name: ReactNode }) {
  const title = typeof name === "string" ? name : undefined;
  return (
    <span className="min-w-0 flex-1 truncate" title={title}>
      {name}
    </span>
  );
}

export function CompactItem({ item }: { item: Item }) {
  const pathname = usePathname();
  const depth = useFolderDepth();
  const topLevel = depth === 0;
  return (
    <SidebarItem
      href={item.url}
      external={item.external}
      active={isActive(item.url, pathname)}
      icon={topLevel ? item.icon : undefined}
      className={cn(ITEM_CLS, !topLevel && "docs-sb-item--nested")}
    >
      <SidebarLabel name={item.name} />
    </SidebarItem>
  );
}

export function CompactFolder({ item, children }: { item: Folder; children: ReactNode }) {
  const pathname = usePathname();
  const depth = useFolderDepth();
  const treePath = useTreePath();
  const inPath = treePath.includes(item);
  const topLevel = depth === 0;
  const persistKey = folderPersistKey(item);
  const headerCls = cn(ITEM_CLS, "docs-sb-folder-head w-full text-start");
  const icon = topLevel ? item.icon : undefined;

  return (
    <SidebarFolder
      className="docs-sb-folder"
      collapsible={item.collapsible}
      defaultOpen={inPath || (item.defaultOpen ?? false)}
      active={inPath}
    >
      <FolderOpenPersistWriter folderKey={persistKey} />
      {item.index && !hasHoistedOverview(item) ? (
        <SidebarFolderLink
          href={item.index.url}
          active={isActive(item.index.url, pathname)}
          external={item.index.external}
          className={headerCls}
        >
          {icon}
          <SidebarLabel name={item.name} />
        </SidebarFolderLink>
      ) : (
        <SidebarFolderTrigger className={headerCls}>
          {icon}
          <SidebarLabel name={item.name} />
        </SidebarFolderTrigger>
      )}
      <SidebarFolderContent className="docs-sb-kids-clip overflow-hidden">
        <div className="docs-sb-kids">{children}</div>
      </SidebarFolderContent>
    </SidebarFolder>
  );
}
