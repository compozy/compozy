import { AlertTriangle, ChevronRight } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@compozy/ui";

import type { AgentPayload } from "../types";

export interface AgentDiagnosticsBannerProps {
  diagnostics: NonNullable<AgentPayload["diagnostics"]>;
}

/** The plain message leads; the file location and error kind stay one click deeper. */
export function AgentDiagnosticsBanner({ diagnostics }: AgentDiagnosticsBannerProps) {
  if (diagnostics.length === 0) return null;
  return (
    <Alert variant="warning" data-testid="agent-diagnostics-banner" role="status">
      <AlertTriangle className="size-4" aria-hidden="true" />
      <AlertTitle>This agent's file has errors</AlertTitle>
      <AlertDescription>
        <ul className="mt-2 flex flex-col gap-1">
          {diagnostics.map(item => (
            <li key={`${item.path}:${item.error_kind}:${item.message}`}>{item.message}</li>
          ))}
        </ul>
        <Collapsible className="mt-2">
          <CollapsibleTrigger
            className="group/agent-diagnostics"
            render={
              <Button
                className="-ml-2 gap-1"
                data-testid="agent-diagnostics-details"
                size="sm"
                type="button"
                variant="ghost"
              />
            }
          >
            <ChevronRight
              aria-hidden="true"
              className="size-3.5 transition-transform group-data-panel-open/agent-diagnostics:rotate-90"
            />
            Details
          </CollapsibleTrigger>
          <CollapsibleContent>
            <ul className="mt-1 flex flex-col gap-1.5">
              {diagnostics.map(item => (
                <li
                  className="font-mono text-mono-id text-muted"
                  data-testid="agent-diagnostic-item"
                  key={`${item.path}:${item.error_kind}:${item.message}`}
                >
                  {item.path ? `${item.path} · ` : null}
                  {item.error_kind}
                </li>
              ))}
            </ul>
          </CollapsibleContent>
        </Collapsible>
      </AlertDescription>
    </Alert>
  );
}
