import { Button } from "@agh/ui";

import { RuntimeSelector } from "@/systems/runtime";

import type { AgentPayload } from "../types";
import { useAgentRuntimeEditor } from "../hooks/use-agent-runtime-editor";

export interface AgentRuntimeControlProps {
  agent: AgentPayload;
  workspaceId: string | null;
  /**
   * id of a visible caption that names the selector. When omitted, an
   * sr-only "Agent runtime" label is rendered for standalone mounts.
   */
  labelledBy?: string;
}

/**
 * Immediate Provider · Model · Reasoning editor for agent detail. Surfaces
 * pending/conflict/error feedback above the selector inside the Runtime card.
 */
export function AgentRuntimeControl({ agent, workspaceId, labelledBy }: AgentRuntimeControlProps) {
  const runtime = useAgentRuntimeEditor({ agent, workspaceId });
  const labelId = labelledBy ?? "agent-detail-runtime-label";
  const statusMessage = runtime.isPending
    ? { tone: "muted" as const, testId: "agent-detail-runtime-pending", text: "Updating runtime…" }
    : runtime.conflictMessage
      ? {
          tone: "warning" as const,
          testId: "agent-detail-runtime-conflict",
          text: runtime.conflictMessage,
        }
      : runtime.error
        ? { tone: "danger" as const, testId: "agent-detail-runtime-error", text: runtime.error }
        : runtime.providerSourceError
          ? {
              tone: "danger" as const,
              testId: "agent-detail-runtime-providers-error",
              text: runtime.providerSourceError,
            }
          : runtime.modelCatalogError
            ? {
                tone: "warning" as const,
                testId: "agent-detail-runtime-catalog-error",
                text: runtime.modelCatalogError,
              }
            : null;

  return (
    <div className="flex min-w-0 flex-col items-start gap-2" data-testid="agent-detail-runtime">
      {statusMessage || runtime.providerSourceError ? (
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          {statusMessage ? (
            <span
              className={
                statusMessage.tone === "danger"
                  ? "text-small-body text-danger"
                  : statusMessage.tone === "warning"
                    ? "text-small-body text-warning"
                    : "text-small-body text-muted"
              }
              data-testid={statusMessage.testId}
              role={statusMessage.tone === "danger" ? "alert" : "status"}
            >
              {statusMessage.text}
            </span>
          ) : null}
          {runtime.providerSourceError ? (
            <Button
              data-testid="agent-detail-runtime-providers-retry"
              onClick={runtime.onRetryProviderSource}
              size="sm"
              type="button"
              variant="ghost"
            >
              Retry providers
            </Button>
          ) : null}
        </div>
      ) : null}
      <RuntimeSelector
        ariaLabelledby={labelId}
        catalogLoaded={runtime.modelCatalogLoaded}
        disabled={
          runtime.providersLoading || runtime.providerOptions.length === 0 || runtime.isPending
        }
        loading={runtime.modelCatalogLoading}
        models={runtime.runtimeModels}
        onChange={runtime.onChange}
        onOpenProviderSettings={runtime.onOpenProviderSettings}
        onRefreshCatalog={runtime.onRefreshCatalog}
        providers={runtime.providerOptions}
        refreshing={runtime.modelCatalogRefreshing}
        triggerId="agent-detail-runtime-trigger"
        triggerTestId="agent-detail-runtime-select"
        value={runtime.value}
        variant="default"
      />
      {labelledBy ? null : (
        <span className="sr-only" id={labelId}>
          Agent runtime
        </span>
      )}
    </div>
  );
}
