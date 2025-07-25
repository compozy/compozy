import { Icon } from "@/components/ui/icon";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import React from "react";
import { tv, type VariantProps } from "tailwind-variants";

const list = tv({
  slots: {
    container: "not-prose my-8",
    list: "flex flex-col",
  },
  variants: {
    size: {
      sm: {
        list: "gap-2",
      },
      md: {
        list: "gap-3",
      },
      lg: {
        list: "gap-4",
      },
    },
    title: {
      true: {},
      false: {},
    },
  },
  compoundVariants: [
    {
      size: "sm",
      title: true,
      class: {
        list: "gap-4",
      },
    },
    {
      size: "md",
      title: true,
      class: {
        list: "gap-5",
      },
    },
    {
      size: "lg",
      title: true,
      class: {
        list: "gap-6",
      },
    },
  ],
  defaultVariants: {
    size: "md",
    title: false,
  },
});

const listItem = tv({
  slots: {
    container: "grid grid-cols-[auto_1fr] gap-2 rounded-md transition-colors",
    indicator: "flex justify-center items-center",
    number: "text-muted-foreground/50 flex items-center justify-center font-display",
    icon: "text-muted-foreground/50",
    content: "flex flex-col justify-center",
    title: "font-semibold text-foreground",
    description: "text-muted-foreground",
  },
  variants: {
    size: {
      sm: {
        container: "gap-1.5",
        indicator: "mt-0 w-8",
        number: "w-8 text-xl",
        icon: "w-4",
        content: "gap-0.5",
        title: "text-sm",
        description: "text-sm leading-tight",
      },
      md: {
        container: "gap-2",
        indicator: "mt-0.5 w-12",
        number: "w-12 text-3xl",
        icon: "w-6",
        content: "gap-0.5",
        title: "text-base",
        description: "text-base",
      },
      lg: {
        container: "gap-2.5",
        indicator: "mt-0.5 w-14",
        number: "w-14 text-4xl",
        icon: "w-7",
        content: "gap-1",
        title: "text-lg",
        description: "text-base",
      },
    },
  },
  defaultVariants: {
    size: "md",
  },
});

interface ListProps extends VariantProps<typeof list> {
  children: React.ReactNode;
  icon?: LucideIcon | string;
  className?: string;
  size?: "sm" | "md" | "lg";
}

interface ListItemProps extends VariantProps<typeof listItem> {
  title?: string;
  children: React.ReactNode;
  className?: string;
  index?: number;
  icon?: LucideIcon | string;
  size?: "sm" | "md" | "lg";
}

export function List({ children, icon, className, size = "md" }: ListProps) {
  const title = React.Children.toArray(children).some(child => {
    if (React.isValidElement(child) && child.type === ListItem) {
      return Boolean((child.props as ListItemProps).title);
    }
    return false;
  });
  const styles = list({ size, title });
  const itemsWithProps = React.Children.map(children, (child, index) => {
    if (React.isValidElement(child) && child.type === ListItem) {
      return React.cloneElement(child as React.ReactElement<ListItemProps>, {
        index: index + 1,
        icon: (child as React.ReactElement<ListItemProps>).props.icon || icon,
        size: (child as React.ReactElement<ListItemProps>).props.size || size,
      });
    }
    return child;
  });

  return (
    <div className={cn(styles.container(), className)}>
      <ul className={styles.list()}>{itemsWithProps}</ul>
    </div>
  );
}

export function ListItem({ title, children, className, index, icon, size = "md" }: ListItemProps) {
  const styles = listItem({ size });

  return (
    <li className={cn(styles.container(), className)}>
      <div className={styles.indicator()}>
        {icon ? (
          typeof icon === "string" ? (
            <Icon name={icon} className={styles.icon()} strokeWidth={1} />
          ) : (
            React.createElement(icon, {
              className: styles.icon(),
              strokeWidth: 1,
            })
          )
        ) : (
          <span className={styles.number()}>{index}</span>
        )}
      </div>
      <div className={styles.content()}>
        {title && <div className={styles.title()}>{title}</div>}
        <div className={styles.description()}>{children}</div>
      </div>
    </li>
  );
}
