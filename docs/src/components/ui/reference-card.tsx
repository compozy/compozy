import { MagicCard } from "@/components/magicui/magic-card";
import { cn } from "@/lib/utils";
import type { LucideIcon } from "lucide-react";
import { ArrowRight } from "lucide-react";
import React from "react";
import { Icon } from "@/components/ui/icon";
import { tv, type VariantProps } from "tailwind-variants";

const referenceCard = tv({
  slots: {
    container: "block no-underline group cursor-pointer",
    card: "not-prose font-normal relative ring-1 ring-transparent rounded-md bg-card overflow-hidden transition-colors h-full",
    content: "p-4",
    icon: "text-muted-foreground/50 transition-colors group-hover:text-primary",
    textContent: "flex flex-col gap-1",
    title: "font-semibold text-foreground !text-md !mb-0",
    description: "text-muted-foreground text-sm",
    chevron:
      "text-muted-foreground/50 transition-transform group-hover:translate-x-1 group-hover:text-primary",
  },
  variants: {
    hasIcon: {
      true: {
        content: "grid grid-cols-[auto_1fr_auto] grid-rows-2 gap-x-4 items-center p-4",
        icon: "row-span-2 text-muted-foreground/50",
        textContent: "col-start-2 row-span-2 flex flex-col gap-1",
        chevron:
          "row-span-2 col-start-3 text-muted-foreground/50 transition-transform group-hover:translate-x-1 group-hover:text-primary",
      },
      false: {
        content: "flex items-center justify-between p-4",
        textContent: "flex flex-col gap-1",
        chevron:
          "ml-4 flex-shrink-0 text-muted-foreground/50 transition-transform group-hover:translate-x-1 group-hover:text-primary",
      },
    },
  },
  defaultVariants: {
    hasIcon: false,
  },
});

interface ReferenceCardProps extends VariantProps<typeof referenceCard> {
  title: string;
  description: string;
  icon?: LucideIcon | string;
  href?: string;
  className?: string;
  onClick?: () => void;
}

export function ReferenceCard({
  title,
  description,
  icon: IconComponent,
  href,
  className,
  onClick,
}: ReferenceCardProps) {
  const styles = referenceCard({ hasIcon: !!IconComponent });

  const content = (
    <div className={styles.content()}>
      {IconComponent && (
        typeof IconComponent === "string" ? (
          <Icon 
            name={IconComponent} 
            className={cn(styles.icon(), "size-8")} 
            strokeWidth={1} 
          />
        ) : (
          <IconComponent className={cn(styles.icon(), "size-8")} strokeWidth={1} />
        )
      )}
      <div className={styles.textContent()}>
        <div className={styles.title()}>{title}</div>
        <div className={styles.description()}>{description}</div>
      </div>
      <ArrowRight className={cn(styles.chevron(), "size-8")} strokeWidth={1} />
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

  if (onClick) {
    return (
      <div onClick={onClick} className={styles.container()}>
        {cardContent}
      </div>
    );
  }

  return cardContent;
}

const referenceCardList = tv({
  base: "not-prose grid grid-cols-1 gap-4 my-6",
});

interface ReferenceCardListProps {
  children: React.ReactNode;
  className?: string;
}

export function ReferenceCardList({ children, className }: ReferenceCardListProps) {
  return <div className={referenceCardList({ className })}>{children}</div>;
}
