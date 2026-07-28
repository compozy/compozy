import * as React from "react";

import { toneText } from "../../lib/tone";
import { cn } from "../../lib/utils";

export type CatalogCardTone = "accent" | "neutral" | "success" | "warning" | "danger" | "info";

/** Logo well size for browse and configured-provider surfaces. */
export type CatalogCardLogoSize = "default" | "lg";

export interface CatalogCardLogoProps extends React.ComponentProps<"span"> {
  tone?: CatalogCardTone;
  size?: CatalogCardLogoSize;
}

export type CatalogCardTitleProps = React.ComponentProps<"h3">;
export type CatalogCardDescriptionProps = React.ComponentProps<"p">;
export type CatalogCardMetaProps = React.ComponentProps<"div">;
export type CatalogCardActionsProps = React.ComponentProps<"div">;

const LOGO_SIZE_CLASS: Record<CatalogCardLogoSize, string> = {
  default: "size-(--size-catalog-logo)",
  lg: "size-(--size-provider-logo-well)",
};

export function CatalogCardLogo({
  tone = "accent",
  size = "default",
  className,
  ...props
}: CatalogCardLogoProps) {
  return (
    <span
      aria-hidden="true"
      data-slot="catalog-card-logo"
      data-tone={tone}
      data-size={size}
      className={cn(
        "inline-flex shrink-0 items-center justify-center rounded bg-surface-glaze",
        LOGO_SIZE_CLASS[size],
        toneText(tone),
        className
      )}
      {...props}
    />
  );
}

export function CatalogCardTitle({ className, ...props }: CatalogCardTitleProps) {
  return (
    <h3
      data-slot="catalog-card-title"
      className={cn(
        "min-w-0 truncate text-small-body font-medium tracking-modal-title text-fg-strong",
        className
      )}
      {...props}
    />
  );
}

export function CatalogCardDescription({ className, ...props }: CatalogCardDescriptionProps) {
  return (
    <p
      data-slot="catalog-card-description"
      className={cn("text-small-body leading-6 text-muted", className)}
      {...props}
    />
  );
}

export function CatalogCardMeta({ className, ...props }: CatalogCardMetaProps) {
  return (
    <div
      data-slot="catalog-card-meta"
      className={cn("eyebrow flex flex-wrap items-center gap-2 text-muted", className)}
      {...props}
    />
  );
}

export function CatalogCardActions({ className, ...props }: CatalogCardActionsProps) {
  return (
    <div
      data-slot="catalog-card-actions"
      className={cn(
        "mt-auto flex flex-wrap items-center gap-2 border-t border-line pt-3",
        className
      )}
      {...props}
    />
  );
}
