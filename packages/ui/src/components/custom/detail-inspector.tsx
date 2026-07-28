"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "../sheet";
import { Tabs, TabsList, TabsTrigger } from "../tabs";
import { useInlineLayout } from "./hooks/use-inline-layout";

export const DETAIL_INSPECTOR_INLINE_BREAKPOINT = 1440;
export const DETAIL_INSPECTOR_INLINE_WIDTH = 320;

export interface DetailInspectorTab {
  /** Stable identifier used as the controlled value. */
  id: string;
  /** Tab label rendered in the trigger row. */
  label: React.ReactNode;
}

export interface DetailInspectorProps {
  /** Optional title rendered in the inline header and the drawer sheet header. */
  title?: React.ReactNode;
  /** Optional tab strip rendered above the body. */
  tabs?: ReadonlyArray<DetailInspectorTab>;
  /** Controlled active tab id. */
  activeTab?: string;
  /** Notified when the user selects a tab. */
  onTabChange?: (id: string) => void;
  /** Body region rendered for the active tab. */
  children: React.ReactNode;
  /** Custom breakpoint in pixels. Defaults to 1440. */
  inlineBreakpoint?: number;
  /** Drawer open state when the viewport is below `inlineBreakpoint`. */
  open?: boolean;
  /** Drawer open-state change handler. */
  onOpenChange?: (open: boolean) => void;
  /** Optional className forwarded to the inline panel root. */
  className?: string;
  /** Optional className forwarded to the Sheet content shell in drawer mode. */
  drawerClassName?: string;
}

function DetailInspectorBody({
  tabs,
  activeTab,
  onTabChange,
  children,
}: Pick<DetailInspectorProps, "tabs" | "activeTab" | "onTabChange" | "children">) {
  return (
    <>
      {tabs && tabs.length > 0 ? (
        <Tabs
          data-slot="detail-inspector-tabs"
          value={activeTab}
          onValueChange={value => {
            if (typeof value === "string") onTabChange?.(value);
          }}
        >
          <TabsList>
            {tabs.map(tab => (
              <TabsTrigger key={tab.id} value={tab.id}>
                {tab.label}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      ) : null}
      <div data-slot="detail-inspector-body" className="flex min-h-0 flex-1 flex-col overflow-auto">
        {children}
      </div>
    </>
  );
}

function DetailInspector({
  title,
  tabs,
  activeTab,
  onTabChange,
  children,
  inlineBreakpoint = DETAIL_INSPECTOR_INLINE_BREAKPOINT,
  open,
  onOpenChange,
  className,
  drawerClassName,
}: DetailInspectorProps) {
  const inline = useInlineLayout(inlineBreakpoint);

  if (inline) {
    return (
      <aside
        data-slot="detail-inspector"
        data-mode="inline"
        aria-label={typeof title === "string" ? title : undefined}
        className={cn(
          "flex h-full min-h-0 shrink-0 flex-col border-l border-line bg-canvas-soft",
          className
        )}
        style={{ width: DETAIL_INSPECTOR_INLINE_WIDTH }}
      >
        {title ? (
          <header
            data-slot="detail-inspector-header"
            className="flex shrink-0 items-center gap-2 border-b border-line px-4 py-3 text-small-body font-medium text-fg-strong"
          >
            {title}
          </header>
        ) : null}
        <DetailInspectorBody tabs={tabs} activeTab={activeTab} onTabChange={onTabChange}>
          {children}
        </DetailInspectorBody>
      </aside>
    );
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="right"
        data-slot="detail-inspector"
        data-mode="drawer"
        className={cn("w-(--detail-inspector-width) sm:max-w-none", drawerClassName)}
        style={
          {
            "--detail-inspector-width": `${DETAIL_INSPECTOR_INLINE_WIDTH}px`,
          } as React.CSSProperties
        }
      >
        {title ? (
          <SheetHeader>
            <SheetTitle data-slot="detail-inspector-header">{title}</SheetTitle>
          </SheetHeader>
        ) : null}
        <DetailInspectorBody tabs={tabs} activeTab={activeTab} onTabChange={onTabChange}>
          {children}
        </DetailInspectorBody>
      </SheetContent>
    </Sheet>
  );
}

export { DetailInspector };
