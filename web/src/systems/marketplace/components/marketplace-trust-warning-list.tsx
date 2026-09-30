import { CircleAlert, Info, TriangleAlert } from "lucide-react";

import { Alert, AlertDescription, AlertTitle, cn } from "@compozy/ui";

interface MarketplaceTrustWarningItem {
  id: string;
  severity: string;
  title: string;
  message: string;
  suggested_command?: string | null;
}

interface MarketplaceTrustWarningListProps {
  items: readonly MarketplaceTrustWarningItem[];
  className?: string;
}

const SEVERITY_ICON = { danger: CircleAlert, info: Info, warning: TriangleAlert } as const;

/**
 * Severity-toned findings (trust warnings, diagnostics) as a stack of `Alert`s. The icon carries
 * the severity, so no severity word is repeated in the row.
 */
function MarketplaceTrustWarningList({ items, className }: MarketplaceTrustWarningListProps) {
  if (items.length === 0) return null;
  return (
    <ul className={cn("flex flex-col gap-2", className)}>
      {items.map(item => {
        const tone = severityTone(item.severity);
        const Icon = SEVERITY_ICON[tone];
        return (
          <li key={item.id}>
            <Alert data-severity={item.severity} role="note" variant={tone}>
              <Icon aria-hidden="true" />
              <AlertTitle>{item.title}</AlertTitle>
              <AlertDescription>
                <p>{item.message}</p>
                {item.suggested_command ? (
                  <code className="inline-flex rounded-sm bg-sunken px-2 py-1 font-mono text-form-hint text-fg">
                    {item.suggested_command}
                  </code>
                ) : null}
              </AlertDescription>
            </Alert>
          </li>
        );
      })}
    </ul>
  );
}

function severityTone(severity: string): keyof typeof SEVERITY_ICON {
  if (severity === "error") return "danger";
  if (severity === "info") return "info";
  return "warning";
}

export { MarketplaceTrustWarningList };
export type { MarketplaceTrustWarningItem, MarketplaceTrustWarningListProps };
