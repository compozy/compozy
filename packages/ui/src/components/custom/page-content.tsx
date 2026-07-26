"use client";

import * as React from "react";

import { cn } from "../../lib/utils";

/** Shared main-pane gutter — identical horizontal inset for every route family. */
const PAGE_CONTENT_GUTTER = "mx-auto w-full max-w-content-max px-9";

type PageContentDensity = "listing" | "comfortable" | "compact" | "route";

type PageContentProps = React.ComponentProps<"div"> & {
  density?: PageContentDensity;
};

function pageContentVertical(density: PageContentDensity): string {
  switch (density) {
    case "listing":
      return "flex min-h-full flex-col gap-5 pt-7 pb-20";
    case "compact":
      return "flex flex-col gap-4 pt-3 pb-8 md:gap-5 md:pb-10";
    case "route":
      return "flex min-h-full flex-col gap-5 pt-7 pb-10 md:gap-6 md:pb-12";
    case "comfortable":
      return "flex min-h-full flex-col gap-6 pt-7 pb-12 md:gap-8 md:pb-16";
  }
}

/**
 * Canonical page content container. ListingPage and PageShell compose this so
 * main-pane routes share one horizontal gutter (`px-9` + `max-w-content-max`).
 */
function PageContent({ density = "listing", className, children, ...props }: PageContentProps) {
  return (
    <div
      data-slot="page-content"
      data-density={density}
      className={cn(PAGE_CONTENT_GUTTER, pageContentVertical(density), className)}
      {...props}
    >
      {children}
    </div>
  );
}

export { PageContent, PAGE_CONTENT_GUTTER };
export type { PageContentProps, PageContentDensity };
