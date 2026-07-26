import { Eyebrow, JsonViewer } from "@agh/ui";

import type { LoopTargetProjection } from "../lib/automation-target";

interface AutomationTargetDetailsProps {
  showInputMapping?: boolean;
  target: LoopTargetProjection;
}

function JsonField({ label, value }: { label: string; value: Record<string, unknown> }) {
  const hasValue = Object.keys(value).length > 0;
  return (
    <div>
      <Eyebrow className="text-muted">{label}</Eyebrow>
      {hasValue ? (
        <JsonViewer className="mt-1.5 bg-rail" value={value} />
      ) : (
        <p className="mt-1 text-xs text-subtle">None</p>
      )}
    </div>
  );
}

/** Read-only projection of the Loop branch of an automation target. */
export function AutomationTargetDetails({
  showInputMapping = true,
  target,
}: AutomationTargetDetailsProps) {
  return (
    <div className="space-y-3" data-testid="automation-target-details">
      <dl className="grid gap-3 sm:grid-cols-2">
        <div>
          <dt className="eyebrow text-muted">Loop</dt>
          <dd className="mt-1 font-mono text-small-body text-fg">
            {target.loopName || "Not selected"}
          </dd>
        </div>
        <div>
          <dt className="eyebrow text-muted">Workspace</dt>
          <dd className="mt-1 font-mono text-small-body text-fg">
            {target.workspaceId || "Not selected"}
          </dd>
        </div>
      </dl>
      <JsonField label="Static inputs" value={target.inputs} />
      {showInputMapping ? (
        <JsonField label="Event input mapping" value={target.inputMapping} />
      ) : null}
    </div>
  );
}
