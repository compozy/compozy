import { Code } from "lucide-react";

import { JsonViewer } from "@compozy/ui";

import {
  automationRequestPayloadForDisplay,
  type AutomationRequestProjection,
} from "../../../lib/automation-requests";
import { PreviewCard } from "./preview-card";

/** Exact normalized request the save sends; write-only values render redacted. */
export function AutomationRequestPayload({
  request,
}: {
  request: AutomationRequestProjection<unknown>;
}) {
  return (
    <div data-testid="automation-request-payload">
      <PreviewCard
        icon={Code}
        label="Request · write-only values redacted"
        right={
          <span className="min-w-0 break-all text-right font-mono text-form-hint text-subtle">
            {request.method} {request.path}
          </span>
        }
      >
        <JsonViewer
          className="bg-rail p-3 leading-relaxed"
          value={automationRequestPayloadForDisplay(request.payload)}
        />
      </PreviewCard>
    </div>
  );
}
