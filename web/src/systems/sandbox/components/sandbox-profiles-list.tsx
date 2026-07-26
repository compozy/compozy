import { Boxes } from "lucide-react";

import { Button, DataSurface, Empty, type ListingViewMode } from "@agh/ui";

import type { SettingsSandboxEntry } from "@/systems/settings";
import { SandboxProfileCard } from "./sandbox-profile-card";
import { SandboxProfileRow } from "./sandbox-profile-row";

export interface SandboxProfilesListProps {
  profiles: SettingsSandboxEntry[];
  view?: ListingViewMode;
  selectedName?: string | null;
  isLoading?: boolean;
  error?: Error | null;
  hasActiveFilters?: boolean;
  onSelect?: (entry: SettingsSandboxEntry) => void;
  onEdit?: (entry: SettingsSandboxEntry) => void;
  onDelete?: (entry: SettingsSandboxEntry) => void;
  onClearFilters?: () => void;
  onCreate?: () => void;
  emptyTitle?: string;
  emptyDescription?: string;
  "data-testid"?: string;
}

export function SandboxProfilesList({
  profiles,
  view = "rows",
  selectedName = null,
  isLoading = false,
  error = null,
  hasActiveFilters = false,
  onSelect,
  onEdit,
  onDelete,
  onClearFilters,
  onCreate,
  emptyTitle = "No sandbox profiles defined",
  emptyDescription = 'Use "New sandbox profile" to create an overlay profile referenceable by workspaces.',
  "data-testid": testId = "sandbox-page-list",
}: SandboxProfilesListProps) {
  const showNoMatch = !isLoading && !error && profiles.length === 0 && hasActiveFilters;
  const showEmpty = !isLoading && !error && profiles.length === 0 && !hasActiveFilters;

  if (showNoMatch) {
    return (
      <Empty
        action={
          onClearFilters ? (
            <Button
              data-testid={`${testId}-clear-filters`}
              onClick={onClearFilters}
              size="sm"
              type="button"
              variant="outline"
            >
              Clear filters
            </Button>
          ) : undefined
        }
        data-testid={`${testId}-no-match`}
        description="Try a different search or clear filters."
        icon={Boxes}
        title="No profiles match"
      />
    );
  }

  if (showEmpty) {
    return (
      <Empty
        action={
          onCreate ? (
            <Button
              data-testid={`${testId}-empty-create`}
              onClick={onCreate}
              size="sm"
              type="button"
            >
              New sandbox profile
            </Button>
          ) : undefined
        }
        data-testid="sandbox-page-empty"
        description={emptyDescription}
        icon={Boxes}
        title={emptyTitle}
      />
    );
  }

  return (
    <DataSurface
      className="flex min-h-0 flex-1 flex-col"
      state={isLoading ? "loading" : error ? "error" : "ready"}
    >
      <DataSurface.Loading data-testid={`${testId}-loading`} label="Loading sandbox profiles" />
      <DataSurface.Error
        data-testid={`${testId}-error`}
        description={error?.message}
        icon={Boxes}
        title="Unable to load sandbox profiles"
      />
      <DataSurface.Content className="min-w-0" data-testid={testId}>
        {view === "cards" ? (
          <div
            className="grid grid-cols-1 gap-2.5 sm:grid-cols-2 xl:grid-cols-3"
            data-testid={`${testId}-card-grid`}
          >
            {profiles.map(entry => (
              <SandboxProfileCard
                key={entry.name}
                entry={entry}
                onDelete={onDelete}
                onEdit={onEdit}
                onSelect={onSelect}
                selected={selectedName === entry.name}
              />
            ))}
          </div>
        ) : (
          <div
            className="overflow-hidden rounded-lg border border-line bg-canvas-soft"
            data-testid={`${testId}-rows`}
          >
            {profiles.map(entry => (
              <SandboxProfileRow
                key={entry.name}
                entry={entry}
                onDelete={onDelete}
                onEdit={onEdit}
                onSelect={onSelect}
                selected={selectedName === entry.name}
              />
            ))}
          </div>
        )}
      </DataSurface.Content>
    </DataSurface>
  );
}
