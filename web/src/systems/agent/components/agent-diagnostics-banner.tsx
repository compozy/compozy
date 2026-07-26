import { AlertTriangle } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@agh/ui";

import type { AgentPayload } from "../types";

export interface AgentDiagnosticsBannerProps {
  diagnostics: NonNullable<AgentPayload["diagnostics"]>;
}

export function AgentDiagnosticsBanner({ diagnostics }: AgentDiagnosticsBannerProps) {
  if (diagnostics.length === 0) return null;
  return (
    <Alert variant="warning" data-testid="agent-diagnostics-banner" role="status">
      <AlertTriangle className="size-4" aria-hidden="true" />
      <AlertTitle>Agent definition has problems</AlertTitle>
      <AlertDescription>
        <ul className="mt-2 flex flex-col gap-2">
          {diagnostics.map(item => (
            <li
              key={`${item.path}:${item.error_kind}:${item.message}`}
              data-testid="agent-diagnostic-item"
            >
              <span className="font-medium">{item.error_kind}</span>
              {": "}
              {item.message}
              {item.path ? (
                <code className="ml-2 font-mono text-badge tracking-mono text-muted">
                  {item.path}
                </code>
              ) : null}
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  );
}
