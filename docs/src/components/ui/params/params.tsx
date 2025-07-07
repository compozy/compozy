"use client";

import { cn } from "@/lib/utils";
import { ChevronDown } from "lucide-react";
import React from "react";
import { tv, type VariantProps } from "tailwind-variants";
import { Card, CardContent, CardHeader } from "../card";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "../collapsible";
import { ScrollArea } from "../scroll-area";

const paramsVariants = tv({
  slots: {
    header: "group cursor-pointer hover:bg-muted/50 transition-colors",
    headerContent: "flex items-center gap-2 flex-1",
    chevron: "size-4 transition-transform duration-200 group-data-[state=open]:rotate-180",
    bodyContent: "flex flex-col px-4 py-0",
    trigger: "group-data-[state=closed]/collapsible:border-none",
  },
  variants: {
    collapsible: {
      true: {},
      false: {
        header: "cursor-default hover:bg-transparent",
        chevron: "hidden",
      },
    },
  },
  defaultVariants: {
    collapsible: false,
  },
});

export interface ParamsProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof paramsVariants> {
  /** Whether the params should be collapsible */
  collapsible?: boolean;
  /** Whether the params content should be scrollable */
  scrollable?: boolean;
  /** Whether the collapsible is open by default */
  defaultOpen?: boolean;
  /** Whether the collapsible is controlled */
  open?: boolean;
  /** Callback when collapsible state changes */
  onOpenChange?: (open: boolean) => void;
}

export interface ParamsHeaderProps extends React.HTMLAttributes<HTMLElement> {
  /** Header content */
  children: React.ReactNode;
}

export interface ParamsBodyProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Body content */
  children: React.ReactNode;
  /** Whether the body content should be scrollable */
  scrollable?: boolean;
}

// Create a context to track collapsible and scrollable state
const ParamsContext = React.createContext<{ collapsible: boolean; scrollable: boolean }>({
  collapsible: false,
  scrollable: false,
});

/**
 * Params component provides a card-like wrapper for parameter content
 * with optional collapsible functionality and scroll area for large content
 */
function ParamsRoot({
  className,
  collapsible = false,
  scrollable = false,
  defaultOpen = false,
  open,
  onOpenChange,
  children,
  ...props
}: ParamsProps) {
  const contextValue = React.useMemo(
    () => ({ collapsible, scrollable }),
    [collapsible, scrollable]
  );

  if (collapsible) {
    return (
      <ParamsContext.Provider value={contextValue}>
        <Collapsible
          className={cn(className, "group/collapsible")}
          defaultOpen={defaultOpen}
          open={open}
          onOpenChange={onOpenChange}
          {...props}
        >
          <Card className={className}>{children}</Card>
        </Collapsible>
      </ParamsContext.Provider>
    );
  }

  return (
    <ParamsContext.Provider value={contextValue}>
      <Card className={className} {...props}>
        {children}
      </Card>
    </ParamsContext.Provider>
  );
}

/**
 * Params Header component for titles and trigger content
 */
function ParamsHeader({ className, children, ...props }: ParamsHeaderProps) {
  const { collapsible } = React.useContext(ParamsContext);
  const styles = paramsVariants({ collapsible });

  if (collapsible) {
    return (
      <CollapsibleTrigger asChild className={styles.trigger()}>
        <CardHeader className={styles.header({ className })} {...props}>
          <div className={styles.headerContent()}>{children}</div>
          <ChevronDown className={styles.chevron()} />
        </CardHeader>
      </CollapsibleTrigger>
    );
  }

  return (
    <CardHeader className={styles.header({ className })} {...props}>
      <div className={styles.headerContent()}>{children}</div>
    </CardHeader>
  );
}

/**
 * Params Body component for main content with optional scroll area
 */
function ParamsBody({
  className,
  children,
  scrollable: scrollableProp,
  ...props
}: ParamsBodyProps) {
  const { collapsible, scrollable: scrollableContext } = React.useContext(ParamsContext);
  const styles = paramsVariants({ collapsible });
  // Use prop if provided, otherwise fall back to context
  const isScrollable = scrollableProp !== undefined ? scrollableProp : scrollableContext;

  const content = (
    <CardContent className={styles.bodyContent({ className })} {...props}>
      {children}
    </CardContent>
  );

  const wrappedContent = isScrollable ? (
    <ScrollArea className="h-[500px]">{content}</ScrollArea>
  ) : (
    content
  );

  if (collapsible) {
    return <CollapsibleContent asChild>{wrappedContent}</CollapsibleContent>;
  }

  return wrappedContent;
}

// Export the compound component
export const Params = Object.assign(ParamsRoot, {
  Header: ParamsHeader,
  Body: ParamsBody,
});
