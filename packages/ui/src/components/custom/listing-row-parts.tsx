import { mergeProps } from "@base-ui/react/merge-props";
import { useRender } from "@base-ui/react/use-render";
import * as React from "react";

import { cn } from "../../lib/utils";

export type ListingRowLinkProps = useRender.ComponentProps<"a">;
export type ListingRowIconProps = React.ComponentProps<"span">;
export type ListingRowMainProps = React.ComponentProps<"div">;
export type ListingRowDescriptionProps = React.ComponentProps<"p">;
export type ListingRowMetaProps = React.ComponentProps<"div">;
export type ListingRowTrailProps = React.ComponentProps<"div">;
export type ListingRowStatProps = React.ComponentProps<"div">;
export type ListingRowSlugProps = React.ComponentProps<"span">;

export interface ListingRowNameProps extends React.ComponentProps<"div"> {
  mono?: boolean;
}

export interface ListingRowTitleProps extends React.ComponentProps<"b"> {
  mono?: boolean;
}

const ListingRowNameContext = React.createContext(false);

function ListingRowLink({ className, render, ...props }: ListingRowLinkProps) {
  const element = useRender({
    defaultTagName: "a",
    props: mergeProps<"a">(
      {
        className: cn(
          "col-span-2 grid min-w-0 grid-cols-[var(--size-icon-well-row)_minmax(0,1fr)] items-center gap-3.5 rounded-sm outline-none focus-visible:shadow-focus-inset",
          className
        ),
      } as Record<string, unknown>,
      { "data-slot": "listing-row-link" } as Record<string, unknown>,
      props
    ),
    render,
    state: { slot: "listing-row-link" },
  });
  return <>{element}</>;
}

function ListingRowIcon({ className, ...props }: ListingRowIconProps) {
  return (
    <span
      aria-hidden="true"
      data-slot="listing-row-icon"
      className={cn(
        "grid size-icon-well-row shrink-0 place-items-center rounded-md bg-elevated text-muted",
        className
      )}
      {...props}
    />
  );
}

function ListingRowMain({ className, ...props }: ListingRowMainProps) {
  return <div data-slot="listing-row-main" className={cn("min-w-0", className)} {...props} />;
}

function ListingRowName({ mono = false, className, ...props }: ListingRowNameProps) {
  return (
    <ListingRowNameContext.Provider value={mono}>
      <div
        data-slot="listing-row-name"
        data-mono={mono ? "true" : undefined}
        className={cn("flex min-w-0 items-center gap-2", className)}
        {...props}
      />
    </ListingRowNameContext.Provider>
  );
}

function ListingRowTitle({ mono, className, ...props }: ListingRowTitleProps) {
  const nameMono = React.use(ListingRowNameContext);
  const useMono = mono ?? nameMono;
  return (
    <b
      data-slot="listing-row-title"
      data-mono={useMono ? "true" : undefined}
      className={cn(
        "min-w-0 truncate font-medium text-fg-strong",
        useMono
          ? "font-mono text-xs tracking-normal"
          : "font-sans text-card-title tracking-row-title",
        className
      )}
      {...props}
    />
  );
}

function ListingRowSlug({ className, ...props }: ListingRowSlugProps) {
  return (
    <span
      data-slot="listing-row-slug"
      className={cn("shrink-0 whitespace-nowrap font-mono text-eyebrow text-faint", className)}
      {...props}
    />
  );
}

function ListingRowDescription({ className, ...props }: ListingRowDescriptionProps) {
  return (
    <p
      data-slot="listing-row-description"
      className={cn("mt-1 truncate text-small-body text-muted", className)}
      {...props}
    />
  );
}

function ListingRowMeta({ className, ...props }: ListingRowMetaProps) {
  return (
    <div
      data-slot="listing-row-meta"
      className={cn("mt-1.5 flex flex-wrap items-center gap-2 text-eyebrow text-faint", className)}
      {...props}
    />
  );
}

function ListingRowMetaDot({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      aria-hidden="true"
      data-slot="listing-row-meta-dot"
      className={cn("size-0.5 rounded-full bg-faint", className)}
      {...props}
    />
  );
}

function ListingRowTrail({ className, ...props }: ListingRowTrailProps) {
  return (
    <div
      data-slot="listing-row-trail"
      className={cn("flex shrink-0 items-center gap-3", className)}
      {...props}
    />
  );
}

function ListingRowStat({ className, children, ...props }: ListingRowStatProps) {
  return (
    <div
      data-slot="listing-row-stat"
      className={cn("flex flex-col items-end gap-0.5", className)}
      {...props}
    >
      {children}
    </div>
  );
}

function ListingRowStatValue({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="listing-row-stat-value"
      className={cn("font-mono text-xs font-semibold tabular-nums text-fg", className)}
      {...props}
    />
  );
}

function ListingRowStatLabel({ className, ...props }: React.ComponentProps<"span">) {
  return (
    <span
      data-slot="listing-row-stat-label"
      className={cn("text-badge text-faint", className)}
      {...props}
    />
  );
}

export {
  ListingRowDescription,
  ListingRowIcon,
  ListingRowLink,
  ListingRowMain,
  ListingRowMeta,
  ListingRowMetaDot,
  ListingRowName,
  ListingRowSlug,
  ListingRowStat,
  ListingRowStatLabel,
  ListingRowStatValue,
  ListingRowTitle,
  ListingRowTrail,
};
