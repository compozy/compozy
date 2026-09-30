import * as React from "react";

import { cn } from "../../lib/utils";
import {
  CatalogCardActions,
  CatalogCardDescription,
  CatalogCardLogo,
  CatalogCardMeta,
  CatalogCardTitle,
} from "./catalog-card-parts";

interface CatalogCardProps extends React.ComponentProps<"article"> {
  selected?: boolean;
  actionable?: boolean;
}

function CatalogCard({
  selected = false,
  actionable = false,
  className,
  ...props
}: CatalogCardProps) {
  return (
    <article
      data-slot="catalog-card"
      data-selected={selected ? "true" : undefined}
      data-actionable={actionable ? "true" : undefined}
      className={cn(
        "flex min-w-0 flex-col gap-3 rounded-lg bg-canvas p-4 text-fg shadow-card transition-[background-color,box-shadow] duration-base ease-out",
        actionable && "hover:bg-surface-2",
        selected && "bg-selected shadow-inset-strong",
        className
      )}
      {...props}
    />
  );
}

const CatalogCardCompound = Object.assign(CatalogCard, {
  Logo: CatalogCardLogo,
  Title: CatalogCardTitle,
  Description: CatalogCardDescription,
  Meta: CatalogCardMeta,
  Actions: CatalogCardActions,
});

export { CatalogCardCompound as CatalogCard };
export type {
  CatalogCardActionsProps,
  CatalogCardDescriptionProps,
  CatalogCardLogoProps,
  CatalogCardLogoSize,
  CatalogCardMetaProps,
  CatalogCardTitleProps,
  CatalogCardTone,
} from "./catalog-card-parts";
export type { CatalogCardProps };
