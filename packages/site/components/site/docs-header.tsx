"use client";

import { buttonVariants, cn } from "@agh/ui";
import { useNotebookLayout } from "fumadocs-ui/layouts/notebook";
import {
  isLayoutTabActive,
  LinkItem,
  type LayoutTab,
  type LinkItemType,
} from "fumadocs-ui/layouts/shared";
import { Sidebar as SidebarIcon } from "lucide-react";
import type { ComponentProps } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { HeaderSearchInput } from "@/components/site/header-search-input";

type WithUrl = Extract<LinkItemType, { url: string }>;

export function DocsHeader(props: ComponentProps<"header">) {
  const {
    slots,
    navItems,
    isNavTransparent,
    props: { tabMode, nav, tabs, sidebar },
  } = useNotebookLayout();

  const sidebarSlots = slots.sidebar;
  const { open } = sidebarSlots?.useSidebar?.() ?? {};
  const navMode = nav?.mode ?? "auto";
  const sidebarCollapsible = sidebar.collapsible ?? true;
  const showLayoutTabs = tabMode === "navbar" && tabs.length > 0;

  const mainItems = navItems.filter(item => item.type !== "icon");
  const iconItems = navItems.filter(item => item.type === "icon");

  return (
    <header
      id="nd-subnav"
      data-transparent={isNavTransparent && !open}
      {...props}
      className={cn(
        "sticky [grid-area:header] flex flex-col top-(--fd-docs-row-1) z-10 backdrop-blur-sm transition-colors data-[transparent=false]:bg-fd-background/80 layout:[--fd-header-height:--spacing(14)]",
        showLayoutTabs && "lg:layout:[--fd-header-height:--spacing(24)]",
        props.className
      )}
    >
      <div data-header-body="" className="flex border-b px-4 gap-2 h-14 md:px-6 items-center">
        <div
          className={cn(
            "items-center",
            navMode === "top" && "flex",
            navMode === "auto" && "hidden has-data-[collapsed=true]:md:flex max-md:flex"
          )}
        >
          {sidebarCollapsible && sidebarSlots && navMode === "auto" && (
            <sidebarSlots.collapseTrigger
              className={cn(
                buttonVariants({ variant: "ghost", size: "icon-sm" }),
                "-ms-1.5 text-fd-muted-foreground data-[collapsed=false]:hidden max-md:hidden"
              )}
            >
              <SidebarIcon aria-hidden />
            </sidebarSlots.collapseTrigger>
          )}
          {slots.navTitle && (
            <slots.navTitle
              className={cn(
                "inline-flex items-center gap-2.5 font-semibold",
                navMode === "auto" && "md:hidden"
              )}
            />
          )}
          {nav?.children}
        </div>

        <nav className="flex flex-1 items-center justify-start gap-6 empty:hidden max-lg:hidden">
          {mainItems.map((item, index) => (
            <NavbarLinkItem key={getLinkItemKey(item, index)} item={item} />
          ))}
        </nav>

        {slots.searchTrigger && (
          <HeaderSearchInput
            hideIfDisabled
            className={cn(
              "my-auto ms-auto max-md:hidden",
              navMode === "top" ? "ps-2.5 rounded-xl max-w-sm" : "max-w-60"
            )}
          />
        )}

        <div className="flex items-center md:gap-2">
          {iconItems.map((item, index) => {
            const iconItem = item as Extract<LinkItemType, { type: "icon" }>;
            return (
              <LinkItem
                key={getLinkItemKey(iconItem, index)}
                item={iconItem as WithUrl}
                aria-label={iconItem.label}
                className={cn(
                  buttonVariants({ variant: "ghost", size: "icon-sm" }),
                  "text-fd-muted-foreground max-lg:hidden"
                )}
              >
                {iconItem.icon}
              </LinkItem>
            );
          })}

          <div className="flex items-center md:hidden">
            {slots.searchTrigger && <slots.searchTrigger.sm hideIfDisabled className="p-2" />}
            {sidebarSlots && (
              <sidebarSlots.trigger
                className={cn(
                  buttonVariants({
                    variant: "ghost",
                    size: "icon-sm",
                    className: "p-2 -me-1.5",
                  })
                )}
              >
                <SidebarIcon aria-hidden />
              </sidebarSlots.trigger>
            )}
          </div>

          <div className="flex items-center gap-2 max-md:hidden">
            {slots.themeSwitch && <slots.themeSwitch />}
            {sidebarCollapsible && sidebarSlots && navMode === "top" && (
              <sidebarSlots.collapseTrigger
                className={cn(
                  buttonVariants({ variant: "secondary", size: "icon-sm" }),
                  "text-fd-muted-foreground rounded-full -me-1.5"
                )}
              >
                <SidebarIcon aria-hidden />
              </sidebarSlots.collapseTrigger>
            )}
          </div>
        </div>
      </div>

      {showLayoutTabs && <DocsHeaderTabs tabs={tabs} />}
    </header>
  );
}

function DocsHeaderTabs({ tabs }: { tabs: LayoutTab[] }) {
  const pathname = usePathname();
  const selectedIdx = tabs.findLastIndex(option => isLayoutTabActive(option, pathname));

  return (
    <div
      data-header-tabs=""
      className="flex flex-row items-end gap-6 overflow-x-auto border-b px-6 h-10 max-lg:hidden"
    >
      {tabs.map((option, i) => {
        const { title, url, unlisted, props } = option;
        const { className, ...rest } = props ?? {};
        const isSelected = selectedIdx === i;
        return (
          <Link
            key={url}
            href={url}
            className={cn(
              "inline-flex border-b-2 border-transparent transition-colors items-center pb-1.5 font-medium gap-2 text-fd-muted-foreground text-sm text-nowrap hover:text-fd-accent-foreground",
              unlisted && !isSelected && "hidden",
              isSelected && "border-fd-primary text-fd-primary",
              className
            )}
            {...rest}
          >
            {title}
          </Link>
        );
      })}
    </div>
  );
}

function getLinkItemKey(item: LinkItemType, index: number) {
  if ("url" in item && item.url) return item.url;
  if ("label" in item && typeof item.label === "string") return item.label;
  if ("text" in item && typeof item.text === "string") return item.text;
  return `${item.type}-${index}`;
}

function NavbarLinkItem({ item }: { item: LinkItemType }) {
  if (item.type === "custom") return <>{item.children}</>;
  if (item.type === "menu" || !("url" in item) || !item.url) return null;

  return (
    <LinkItem
      item={item as WithUrl}
      className="text-sm text-fd-muted-foreground transition-colors hover:text-fd-accent-foreground data-[active=true]:text-fd-primary"
    >
      {"text" in item ? item.text : null}
    </LinkItem>
  );
}
