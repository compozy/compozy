import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import React from "react";
import { Icon } from "@/components/ui/icon";
import { tv, type VariantProps } from "tailwind-variants";

const list = tv({
  slots: {
    container: "not-prose",
    list: "flex flex-col",
  },
});

const listItem = tv({
  slots: {
    container:
      "py-1 grid grid-cols-[auto_1fr] gap-2 rounded-md transition-colors hover:bg-accent/50",
    indicator: "flex mt-1 justify-center items-center size-14",
    number:
      "size-14 text-muted-foreground/50 flex items-center justify-center text-4xl font-display",
    icon: "text-muted-foreground/50",
    content: "flex flex-col gap-1 justify-center",
    title: "font-semibold text-foreground",
    description: "text-muted-foreground text-sm",
  },
});

interface ListProps extends VariantProps<typeof list> {
  children: React.ReactNode;
  icon?: LucideIcon | string;
  className?: string;
}

interface ListItemProps extends VariantProps<typeof listItem> {
  title?: string;
  children: React.ReactNode;
  className?: string;
  index?: number;
  icon?: LucideIcon | string;
}

export function List({ children, icon, className }: ListProps) {
  const styles = list();

  // Clone children and pass icon and index props
  const itemsWithProps = React.Children.map(children, (child, index) => {
    if (React.isValidElement(child) && child.type === ListItem) {
      return React.cloneElement(child as React.ReactElement<ListItemProps>, {
        index: index + 1,
        icon: (child as React.ReactElement<ListItemProps>).props.icon || icon,
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

export function ListItem({ title, children, className, index, icon }: ListItemProps) {
  const styles = listItem();

  return (
    <li className={cn(styles.container(), className)}>
      <div className={styles.indicator()}>
        {icon ? (
          typeof icon === "string" ? (
            <Icon 
              name={icon} 
              className={cn(styles.icon(), "size-8")}
              strokeWidth={1}
            />
          ) : (
            React.createElement(icon, {
              className: cn(styles.icon(), "size-8"),
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
