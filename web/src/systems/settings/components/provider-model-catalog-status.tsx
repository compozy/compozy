import { RefreshCw } from "@compozy/ui/icons";

import {
  Button,
  Eyebrow,
  Item,
  ItemActions,
  ItemContent,
  ItemGroup,
  ItemTitle,
  Pill,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  Spinner,
  Time,
} from "@compozy/ui";

import {
  modelRefreshStateTone,
  modelSourceFailureMessage,
  modelRefreshErrorMessage,
  type ProviderModelSourceStatus,
  useProviderModelStatus,
  useRefreshProviderModels,
} from "@/systems/model-catalog";

const SOURCE_ITEM_CLASS = "items-start gap-2 rounded-none border-0 p-0";
const SKELETON_SOURCES = ["first", "second"] as const;

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
      <p className="text-form-hint text-subtle" data-testid={`${testId}-disabled`}>
        The model list updates once the app is installed.
      </p>
    );
  }

  if (statusQuery.isLoading) {
    return (
      <SkeletonRegion
        className="flex flex-col gap-3"
        data-testid={`${testId}-loading`}
        label="Loading catalog status"
      >
        <div className="flex flex-col gap-1.5">
          {SKELETON_SOURCES.map(source => (
            <Item aria-hidden="true" className={SOURCE_ITEM_CLASS} key={source} size="xs">
              <ItemContent className="min-w-0 flex-1 gap-1">
                <ItemTitle className="text-small-body text-fg">
                  <SkeletonText className="w-40" />
                </ItemTitle>
                <SkeletonText className="w-28 text-form-hint" />
              </ItemContent>
              <ItemActions className="flex-wrap items-center gap-1.5">
                <Skeleton className="h-pill-sm w-16 rounded-pill" />
                <SkeletonText className="w-16 text-form-hint" />
              </ItemActions>
            </Item>
          ))}
        </div>
        <Skeleton className="h-button-sm w-32 rounded-pill" />
      </SkeletonRegion>
    );
  }

  const sources = statusQuery.data?.sources ?? [];
  const refreshError = modelRefreshErrorMessage(refreshMutation.error, sources);
  const queryError = errorMessage(statusQuery.error);

  const handleRefresh = () => {
    refreshMutation.mutate({ providerId, force: true });
  };

  return (
    <div className="flex flex-col gap-3" data-testid={testId}>
      {queryError ? (
        <p className="text-form-hint text-danger" data-testid={`${testId}-error`}>
          {queryError}
        </p>
      ) : null}
      {sources.length === 0 && !queryError ? (
        <p className="text-form-hint text-subtle" data-testid={`${testId}-empty`}>
          No catalog sources reporting yet.
        </p>
      ) : (
        <ItemGroup className="flex flex-col gap-1.5" data-testid={`${testId}-list`}>
          {sources.map(source => (
            <Item
              key={source.source_id}
              className={SOURCE_ITEM_CLASS}
              size="xs"
              data-testid={`${testId}-source-${source.source_id}`}
            >
              <ItemContent className="min-w-0 flex-1 gap-1">
                <ItemTitle className="text-small-body text-fg">
                  <span className="truncate font-mono">{source.source_id}</span>
                </ItemTitle>
                {modelSourceFailureMessage(source) ? (
                  <p className="text-form-hint text-subtle">{modelSourceFailureMessage(source)}</p>
                ) : null}
                {timestampOf(source) ? (
                  <span className="flex items-center gap-1 text-form-hint text-subtle">
                    <Eyebrow className="text-subtle">refreshed</Eyebrow>
                    <Time iso={timestampOf(source) as string} mode="relative" />
                  </span>
                ) : null}
              </ItemContent>
              <ItemActions className="flex-wrap items-center gap-1.5">
                <Pill
                  mono
                  tone={
                    source.failure_reason === "auth_required"
                      ? "warning"
                      : modelRefreshStateTone(source.refresh_state)
                  }
                >
                  {source.failure_reason === "auth_required" ? "Sign in" : source.refresh_state}
                </Pill>
                {source.stale ? (
                  <Pill mono tone="warning">
                    stale
                  </Pill>
                ) : null}
                <span
                  className="text-form-hint text-muted tabular-nums"
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
        <p className="text-form-hint text-danger" data-testid={`${testId}-refresh-error`}>
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
        {refreshMutation.isPending ? (
          <Spinner aria-hidden="true" className="size-3.5" />
        ) : (
          <RefreshCw aria-hidden="true" />
        )}
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
