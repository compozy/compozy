"use client";

import { cn } from "@/lib/utils";
import { ChevronDown, TableProperties } from "lucide-react";
import React from "react";
import { tv, type VariantProps } from "tailwind-variants";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "../accordion";

// Param component using tailwind-variants with design system tokens
const paramVariants = tv({
  slots: {
    base: "py-4 flex flex-col border-b border-border last:border-b-0",
    header: "flex items-start gap-4",
    content: "",
    path: "font-mono text-sm font-medium bg-transparent flex items-center gap-1",
    type: "text-xs font-medium px-2 py-1 rounded-md bg-accent text-muted-foreground",
    badges: "flex items-center gap-2",
    badge: "text-xs px-2 py-0.5 rounded-full font-medium",
    description: "flex flex-col text-sm text-muted-foreground max-w-none",
    expandButton:
      "flex items-center justify-center w-6 h-6 rounded-md hover:bg-accent transition-colors",
  },
  variants: {
    paramType: {
      query: {
        path: ["text-blue-600", "dark:text-blue-400"],
      },
      path: {
        path: ["text-lime-600", "dark:text-lime-400"],
      },
      body: {
        path: ["text-purple-600", "dark:text-purple-400"],
      },
      header: {
        path: ["text-orange-600", "dark:text-orange-400"],
      },
      response: {
        path: ["text-emerald-600", "dark:text-emerald-400"],
      },
    },
    required: {
      true: {
        badge: ["bg-red-500/10", "text-red-800", "dark:text-red-300"],
      },
      false: {
        badge: ["bg-muted", "text-muted-foreground"],
      },
    },
    deprecated: {
      true: {
        path: ["line-through", "opacity-60"],
        badge: [
          "bg-yellow-100",
          "text-yellow-800",
          "dark:bg-yellow-900/30",
          "dark:text-yellow-300",
        ],
      },
      false: {},
    },
  },
  defaultVariants: {
    paramType: "body",
    required: false,
    deprecated: false,
  },
});

// Types for the param variants
type ParamVariants = VariantProps<typeof paramVariants>;

// Body component variants using tailwind-variants with design system tokens
const bodyVariants = tv({
  slots: {
    base: ["mt-2 text-sm text-muted-foreground prose prose-sm max-w-none"],
  },
});

// Body component for parameter descriptions
interface BodyProps {
  children: React.ReactNode;
  className?: string;
}

function Body({ children, className }: BodyProps) {
  const styles = bodyVariants();
  return <div className={cn(styles.base(), className)}>{children}</div>;
}

// Expandable component variants using tailwind-variants with design system tokens
const expandableVariants = tv({
  slots: {
    root: "space-y-2 mt-4",
    accordionItem: "[&~&]:mt-2 border-0 [&>h3]:m-0 [&>h3]:p-0 [&>h3]:font-sans group",
    accordionTrigger: [
      "text-muted-foreground/50",
      "hover:text-muted-foreground",
      "p-0",
      "text-sm",
      "font-normal",
      "hover:no-underline",
    ],
    accordionContent: "pb-0 mt-2",
    childrenContainer: "flex flex-col border border-border rounded-md [&>[data-slot=param]]:px-4",
    chevron: "size-4 shrink-0 transition-transform duration-200 group-data-[state=open]:rotate-180",
  },
});

// Expandable Root component
interface ExpandableRootProps {
  children: React.ReactNode;
  className?: string;
  type?: "single" | "multiple";
  defaultValue?: string | string[];
}

function ExpandableRoot({
  children,
  className,
  type = "single",
  defaultValue,
}: ExpandableRootProps) {
  const styles = expandableVariants();

  // Handle type-specific props for Accordion
  if (type === "multiple") {
    return (
      <div className={cn(styles.root(), className)}>
        <Accordion type="multiple" defaultValue={defaultValue as string[]} indicator="plus">
          {children}
        </Accordion>
      </div>
    );
  }

  return (
    <div className={cn(styles.root(), className)}>
      <Accordion type="single" collapsible defaultValue={defaultValue as string} indicator="plus">
        {children}
      </Accordion>
    </div>
  );
}

// Expandable Item component
interface ExpandableItemProps {
  title: string;
  children: React.ReactNode;
  value: string;
  icon?: React.ReactNode;
}

function ExpandableItem({
  title,
  children,
  value,
  icon = <TableProperties className="size-3" />,
}: ExpandableItemProps) {
  const styles = expandableVariants();
  return (
    <AccordionItem value={value} className={styles.accordionItem()}>
      <AccordionTrigger hideIndicator className={styles.accordionTrigger()}>
        <div className="flex items-center gap-2">
          {icon}
          {title}
          <ChevronDown className={styles.chevron()} strokeWidth={1} />
        </div>
      </AccordionTrigger>
      <AccordionContent className={styles.accordionContent()}>
        <div className={styles.childrenContainer()}>{children}</div>
      </AccordionContent>
    </AccordionItem>
  );
}

// Main Param component props
export interface ParamProps extends ParamVariants {
  /** Parameter name/path (e.g., "user_id", "query.limit") */
  path?: string;
  /** Parameter type (e.g., "string", "number", "boolean", "object", "array") */
  type?: string;
  /** Whether the parameter is required */
  required?: boolean;
  /** Whether the parameter is deprecated */
  deprecated?: boolean;
  /** Default value for the parameter */
  default?: string;
  /** Initial value for playground/testing */
  initialValue?: any;
  /** Placeholder text for input fields */
  placeholder?: string;
  /** Parameter description (supports MDX content) */
  children?: React.ReactNode;
  /** Additional CSS classes */
  className?: string;
  /** Parameter type for styling (query, path, body, header, response) */
  paramType?: "query" | "path" | "body" | "header" | "response";
}

// Main Param component
export function Param(props: ParamProps) {
  const {
    path: paramPath = "",
    type = "string",
    required = false,
    deprecated = false,
    default: defaultValue,
    children,
    className,
    paramType = "body",
    ...restProps
  } = props;

  const styles = paramVariants({
    paramType,
    required,
    deprecated,
  });

  return (
    <div data-slot="param" className={cn(styles.base(), className)} {...restProps}>
      <div className={styles.header()}>
        <div className="flex-1 min-w-0 flex justify-between items-center">
          <div className="flex items-center gap-3 flex-wrap [&>code]:border-0">
            {paramPath && <code className={styles.path()}>{paramPath} </code>}
            <div className={styles.badges()}>
              {required && <span className={styles.badge({ required: true })}>required</span>}
              {deprecated && <span className={styles.badge({ deprecated: true })}>deprecated</span>}
              {defaultValue && <span className={styles.badge()}>default: {defaultValue}</span>}
            </div>
          </div>
          {type && <span className={styles.type()}>{type}</span>}
        </div>
      </div>

      {children && (
        <div className={styles.content()}>
          <div className={styles.description()}>{children}</div>
        </div>
      )}
    </div>
  );
}

// Compound component exports
Param.Body = Body;
Param.ExpandableRoot = ExpandableRoot;
Param.ExpandableItem = ExpandableItem;

// Export types
export type { BodyProps, ExpandableItemProps, ExpandableRootProps };
