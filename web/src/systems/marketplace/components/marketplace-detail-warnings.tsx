import { TriangleAlert } from "lucide-react";

import { cn, Pill } from "@compozy/ui";

import type { MarketplaceEntryResponse } from "../types";
import { MarketplaceDetailSection } from "./marketplace-detail-shell";

type TrustWarnings = NonNullable<
  NonNullable<MarketplaceEntryResponse["entry"]["trust"]>["warnings"]
>;

interface MarketplaceDetailWarningsProps {
  warnings: TrustWarnings | undefined;
}

/** Trust warnings from the catalog join, one tinted row per finding. */
function MarketplaceDetailWarnings({ warnings }: MarketplaceDetailWarningsProps) {
  if (!warnings || warnings.length === 0) return null;
  return (
    <MarketplaceDetailSection
      data-testid="marketplace-detail-warnings"
      icon={TriangleAlert}
      summary={`${warnings.length} ${warnings.length === 1 ? "finding" : "findings"}`}
      title="Trust warnings"
    >
      <ul className="divide-y divide-line-soft">
        {warnings.map(warning => {
          const tone =
            warning.severity === "error"
              ? "danger"
              : warning.severity === "info"
                ? "info"
                : "warning";
          return (
            <li
              className={cn(
                "px-4 py-3",
                tone === "danger" && "bg-danger-tint",
                tone === "info" && "bg-info-tint",
                tone === "warning" && "bg-warning-tint"
              )}
              key={warning.id}
            >
              <div className="flex items-center gap-2">
                <Pill mono tone={tone}>
                  {warning.severity}
                </Pill>
                <p className="text-small-body font-medium text-fg-strong">{warning.title}</p>
              </div>
              <p className="mt-1.5 text-small-body text-muted">{warning.message}</p>
              {warning.suggested_command ? (
                <code className="mt-2 inline-flex rounded bg-canvas px-2 py-1 font-mono text-form-hint text-fg">
                  {warning.suggested_command}
                </code>
              ) : null}
            </li>
          );
        })}
      </ul>
    </MarketplaceDetailSection>
  );
}

export { MarketplaceDetailWarnings };
export type { MarketplaceDetailWarningsProps };
