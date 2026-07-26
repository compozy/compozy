import { RefreshCw } from "lucide-react";

import {
  Button,
  Eyebrow,
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
  Pill,
  Spinner,
  Time,
} from "@agh/ui";

import {
  modelRefreshStateTone,
  useProviderModelStatus,
  useRefreshProviderModels,
  type ProviderModelSourceStatus,
} from "@/systems/model-catalog";

interface ProviderModelCatalogStatusProps {
  providerId: string;
  testId: string;
  /**
   * When false the hook is skipped; the component still renders a placeholder so
   * callers can show "the binary is missing, nothing to refresh" without rolling
   * their own surface.
   */
  enabled?: boolean;
}

export function ProviderModelCatalogStatus({
  providerId,
  testId,
  enabled = true,
}: ProviderModelCatalogStatusProps) {
  const statusQuery = useProviderModelStatus({ providerId, enabled });
  const refreshMutation = useRefreshProviderModels();

  if (!enabled) {
    return (
      <p className="text-xs text-subtle" data-testid={`${testId}-disabled`}>
        Catalog refresh resumes once the provider binary is available.
      </p>
    );
  }

  if (statusQuery.isLoading) {
    return (
      <div className="flex items-center gap-2 text-xs text-subtle">
        <Spinner className="size-3" />
        <span data-testid={`${testId}-loading`}>Loading catalog status…</span>
      </div>
    );
  }

  const sources = statusQuery.data?.sources ?? [];
  const refreshError = errorMessage(refreshMutation.error);
  const queryError = errorMessage(statusQuery.error);

  const handleRefresh = () => {
    refreshMutation.mutate({ providerId, force: true });
  };

  return (
    <div className="flex flex-col gap-3" data-testid={testId}>
      {queryError ? (
        <p className="text-xs text-danger" data-testid={`${testId}-error`}>
          {queryError}
        </p>
      ) : null}
      {sources.length === 0 && !queryError ? (
        <p className="text-xs text-subtle" data-testid={`${testId}-empty`}>
          No catalog sources reporting yet.
        </p>
      ) : (
        <ItemGroup className="flex flex-col gap-1.5" data-testid={`${testId}-list`}>
          {sources.map(source => (
            <Item
              key={source.source_id}
              className="items-start gap-2 rounded-none border-0 p-0"
              size="xs"
              data-testid={`${testId}-source-${source.source_id}`}
            >
              <ItemContent className="min-w-0 flex-1 gap-1">
                <ItemTitle className="text-small-body text-fg">
                  <span className="truncate font-mono">{source.source_id}</span>
                </ItemTitle>
                {timestampOf(source) ? (
                  <span className="flex items-center gap-1 text-xs text-subtle">
                    <Eyebrow className="text-subtle">refreshed</Eyebrow>
                    <Time iso={timestampOf(source) as string} mode="relative" />
                  </span>
                ) : null}
              </ItemContent>
              <ItemActions className="flex-wrap items-center gap-1.5">
                <Pill mono tone={modelRefreshStateTone(source.refresh_state)}>
                  {source.refresh_state}
                </Pill>
                {source.stale ? (
                  <Pill mono tone="warning">
                    stale
                  </Pill>
                ) : null}
                <span
                  className="text-xs text-muted tabular-nums"
                  data-testid={`${testId}-source-${source.source_id}-rows`}
                >
                  {formatRowCount(source)}
                </span>
              </ItemActions>
            </Item>
          ))}
        </ItemGroup>
      )}
      {refreshError ? (
        <p className="text-xs text-danger" data-testid={`${testId}-refresh-error`}>
          {refreshError}
        </p>
      ) : null}
      <Button
        type="button"
        variant="ghost"
        size="sm"
        className="w-fit"
        onClick={handleRefresh}
        disabled={refreshMutation.isPending || statusQuery.isFetching}
        data-testid={`${testId}-refresh`}
      >
        <RefreshCw
          aria-hidden="true"
          className={refreshMutation.isPending ? "size-3 animate-spin" : "size-3"}
        />
        Refresh catalog
      </Button>
    </div>
  );
}

function formatRowCount(source: ProviderModelSourceStatus): string {
  return `${source.row_count} rows`;
}

function timestampOf(source: ProviderModelSourceStatus): string | undefined {
  return source.last_success?.trim() || source.last_refresh?.trim() || undefined;
}

function errorMessage(error: unknown): string | null {
  if (error instanceof Error && error.message.trim().length > 0) {
    return error.message;
  }
  return null;
}
