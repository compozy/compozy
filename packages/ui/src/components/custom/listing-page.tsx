"use client";

import * as React from "react";

import { cn } from "../../lib/utils";
import { PageContent } from "./page-content";

type ListingPageProps = React.ComponentProps<"div"> & {
  /** Optional banner rendered above the scroll area (e.g. cached-data Alert). */
  banner?: React.ReactNode;
  /** Extra classes for the centered content container. */
  bodyClassName?: string;
};

function ListingPage({ banner, bodyClassName, className, children, ...props }: ListingPageProps) {
  return (
    <div
      data-slot="listing-page"
      className={cn("flex min-h-0 flex-1 flex-col overflow-hidden", className)}
      {...props}
    >
      {banner ? <div data-slot="listing-page-banner">{banner}</div> : null}
      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <PageContent density="listing" className={bodyClassName}>
          {children}
        </PageContent>
      </div>
    </div>
  );
}

export { ListingPage };
export type { ListingPageProps };
