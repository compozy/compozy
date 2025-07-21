import { MagicCard } from "@/components/magicui/magic-card";
import { Icon } from "@/components/ui/icon";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import { ChevronRight } from "lucide-react";
import React from "react";
import { tv, type VariantProps } from "tailwind-variants";

const featureCard = tv({
  slots: {
    container: "block no-underline h-full",
    card: "not-prose font-normal group relative my-2 ring-2 ring-transparent rounded-2xl bg-card border border-border overflow-hidden transition-colors h-full",
    content: "flex flex-col h-full",
    header: "flex flex-col",
    iconWrapper: "flex items-center justify-center rounded-lg bg-primary/10",
    icon: "text-primary",
    title: "font-semibold text-foreground",
    description: "text-muted-foreground flex-1 [&:last-child]:mb-0",
    link: "flex items-center gap-2 font-medium text-primary",
    chevron: "transition-transform group-hover:translate-x-0.5",
  },
  variants: {
    size: {
      sm: {
        content: "p-4",
        header: "gap-2",
        iconWrapper: "w-8 h-8",
        icon: "w-5 h-5",
        title: "!text-base !mb-0",
        description: "my-2 text-sm",
        link: "text-xs",
        chevron: "w-2.5 h-2.5",
      },
      default: {
        content: "p-4 p-6",
        header: "gap-3",
        iconWrapper: "w-8 h-8",
        icon: "w-[18px] h-[18px]",
        title: "text-lg",
        description: "my-4 text-base",
        link: "text-sm",
        chevron: "w-3 h-3",
      },
      lg: {
        content: "p-8",
        header: "gap-4",
        iconWrapper: "w-10 h-10",
        icon: "w-6 h-6",
        title: "text-xl",
        description: "my-4 text-base",
        link: "text-base",
        chevron: "w-4 h-4",
      },
    },
  },
  defaultVariants: {
    size: "default",
  },
});

interface FeatureCardProps extends VariantProps<typeof featureCard> {
  title: string;
  description: string;
  icon?: LucideIcon | string;
  href?: string;
  className?: string;
}

export function FeatureCard({
  title,
  description,
  icon: IconComponent,
  href,
  size,
  className,
}: FeatureCardProps) {
  const styles = featureCard({ size });

  const content = (
    <div className={styles.content()}>
      <div className={styles.header()}>
        {IconComponent && (
          <div className={styles.iconWrapper()}>
            {typeof IconComponent === "string" ? (
              <Icon name={IconComponent} className={styles.icon()} />
            ) : (
              <IconComponent className={styles.icon()} />
            )}
          </div>
        )}
        <h3 className={styles.title()}>{title}</h3>
      </div>

      <div className={styles.description()}>{description}</div>

      {href && (
        <div className={styles.link()}>
          See more
          <ChevronRight className={styles.chevron()} />
        </div>
      )}
    </div>
  );

  const cardContent = (
    <MagicCard
      className={cn(styles.card(), className)}
      gradientSize={200}
      gradientColor="var(--primary)"
      gradientOpacity={0.05}
      gradientFrom="oklch(from var(--primary) calc(l + 0.1) c h)"
      gradientTo="oklch(from var(--primary) calc(l - 0.1) c h)"
      innerClassName="h-full"
    >
      {content}
    </MagicCard>
  );

  if (href) {
    return (
      <a href={href} className={styles.container()}>
        {cardContent}
      </a>
    );
  }

  return cardContent;
}

const featureCardList = tv({
  base: "not-prose grid gap-6 mb-12",
  variants: {
    cols: {
      1: "grid-cols-1",
      2: "grid-cols-1 md:grid-cols-2",
      3: "grid-cols-1 md:grid-cols-2 lg:grid-cols-3",
      4: "grid-cols-1 md:grid-cols-2 lg:grid-cols-4",
    },
    size: {
      sm: "gap-4",
      default: "gap-6",
      lg: "gap-8",
    },
  },
  defaultVariants: {
    cols: 2,
    size: "default",
  },
});

interface FeatureCardListProps extends VariantProps<typeof featureCardList> {
  children: React.ReactNode;
  className?: string;
}

export function FeatureCardList({
  children,
  cols = 2,
  size = "default",
  className,
}: FeatureCardListProps) {
  // Clone children to pass size prop
  const childrenWithSize = React.Children.map(children, child => {
    if (React.isValidElement(child) && child.type === FeatureCard) {
      return React.cloneElement(child as React.ReactElement<FeatureCardProps>, { size });
    }
    return child;
  });

  return <div className={featureCardList({ cols, size, className })}>{childrenWithSize}</div>;
}
