import { FileCode, Lock } from "lucide-react";
import type { ComponentProps } from "react";

import { cn } from "@compozy/ui";

import type { AutomationTrigger } from "../../types";

/**
 * Ownership notice for a trigger the operator cannot edit here.
 *
 * Config- and package-owned triggers accept exactly one change through the API
 * — `enabled` — so Edit and Delete are absent rather than disabled, and this
 * line says where the rest of the definition actually lives.
 */
interface TriggerDetailLockProps extends Omit<ComponentProps<"div">, "children"> {
  trigger: AutomationTrigger;
}

export function TriggerDetailLock({ trigger, className, ...props }: TriggerDetailLockProps) {
  if (trigger.source === "dynamic") return null;
  const copy =
    trigger.source === "config"
      ? "This automation is defined in configuration files. Only its enabled state can be changed here."
      : "This automation is provided by an installed package. Only its enabled state can be changed here.";
  return (
    <div
      className={cn("mb-4.5 flex flex-col gap-2", className)}
      data-testid="trigger-detail-lock"
      {...props}
    >
      <div className="flex items-start gap-2 rounded-md border border-dashed border-line px-4 py-3 text-form-label leading-relaxed text-muted">
        <Lock aria-hidden="true" className="mt-0.5 size-3 shrink-0 text-subtle" />
        <p>{copy}</p>
      </div>
      {trigger.source === "config" ? (
        <div className="flex items-start gap-2 rounded-md border border-line-soft bg-canvas-soft px-3.5 py-3 text-small-body leading-relaxed text-muted">
          <FileCode aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-subtle" />
          <p>
            Lives in <b className="font-medium text-fg">config.toml</b> —{" "}
            <span className="font-mono text-form-hint">
              {`[[automation.triggers]] name = "${trigger.name}"`}
            </span>
          </p>
        </div>
      ) : null}
    </div>
  );
}
