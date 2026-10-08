import { Lock } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

import type { AutomationView } from "../../lib/automation-view";

interface AutomationLockbarProps extends Omit<ComponentProps<"div">, "children"> {
  view: Pick<AutomationView, "kind" | "name" | "source">;
}

/**
 * Ownership notice for an automation the operator cannot edit here.
 *
 * Config- and package-owned automations accept exactly one change through the
 * API — `enabled` — so Edit and Delete are absent rather than disabled, and
 * this bar says where the rest of the definition actually lives.
 */
export function AutomationLockbar({ view, className, ...props }: AutomationLockbarProps) {
  if (view.source === "dynamic") return null;
  const table = view.kind === "job" ? "automation.jobs" : "automation.triggers";
  return (
    <div
      className={cn(
        "mb-4.5 flex items-start gap-2 rounded-md border border-dashed border-line px-4 py-3 text-form-label leading-relaxed text-muted",
        className
      )}
      data-testid="automation-lockbar"
      {...props}
    >
      <Lock aria-hidden="true" className="mt-1 size-3 shrink-0 text-subtle" />
      {view.source === "config" ? (
        <p>
          This automation is defined in configuration files. You can only turn it on or off here.
          Lives in <code className="font-mono text-form-hint text-fg">config.toml</code> —{" "}
          <code className="font-mono text-form-hint text-fg">{`[[${table}]] name = "${view.name}"`}</code>
        </p>
      ) : (
        <p>
          This automation is provided by an installed package. You can only turn it on or off here.
        </p>
      )}
    </div>
  );
}
