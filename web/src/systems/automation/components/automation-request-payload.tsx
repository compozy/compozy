import { Code } from "lucide-react";

import { Alert, AlertDescription, JsonViewer } from "@agh/ui";

import {
  automationRequestPayloadForDisplay,
  type AutomationRequestProjection,
} from "../lib/automation-requests";
import { PreviewCard } from "./trigger-form/preview/preview-card";

interface AutomationRequestPayloadProps {
  blockedReason?: string | null;
  request: AutomationRequestProjection<unknown>;
}

/** Exact normalized request sent by the corresponding create or edit action. */
export function AutomationRequestPayload({
  blockedReason,
  request,
}: AutomationRequestPayloadProps) {
  return (
    <div data-testid="automation-request-payload">
      <PreviewCard
        icon={Code}
        label={blockedReason ? "Request blocked" : "Request payload · write-only values redacted"}
        right={
          <span className="min-w-0 break-all text-right font-mono text-form-hint text-subtle">
            {blockedReason ? "Blocked · " : ""}
            {request.method} {request.path}
          </span>
        }
      >
        {blockedReason ? (
          <Alert className="mb-3" role="alert" variant="warning">
            <AlertDescription>{blockedReason} This request will not be sent.</AlertDescription>
          </Alert>
        ) : null}
        <JsonViewer
          className="bg-rail p-3 leading-relaxed"
          value={automationRequestPayloadForDisplay(request.payload)}
        />
      </PreviewCard>
    </div>
  );
}
