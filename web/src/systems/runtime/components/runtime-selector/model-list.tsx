import { Plus } from "lucide-react";

import { cn, Eyebrow, SkeletonRows } from "@compozy/ui";

import { ModelRow } from "./model-row";
import type {
  GroupAvailability,
  RuntimeGroupModel,
  RuntimeListModel,
} from "./use-runtime-selector";

const AVAILABILITY_TONE: Record<GroupAvailability["tone"], string> = {
  success: "text-success",
  warning: "text-warning",
  danger: "text-danger",
};

/**
 * Compact sticky group head (design ref: runtime-selector-variations.html,
 * variation A): provider name, optional harness badge, and availability as a
 * dot — labelled for AT and on hover, since color alone never carries state.
 */
function GroupHead({
  name,
  harnessBadge,
  availability,
}: {
  name: string;
  harnessBadge?: string;
  availability?: GroupAvailability;
}) {
  return (
    <div
      role="presentation"
      className="sticky top-0 z-[1] flex items-center gap-1.5 bg-canvas px-2 pt-[7px] pb-[3px]"
    >
      <Eyebrow className="text-faint">{name}</Eyebrow>
      {harnessBadge ? (
        <span className="rounded-xxs bg-badge-fill px-[5px] py-px text-faint">
          <Eyebrow>{harnessBadge.toUpperCase()}</Eyebrow>
        </span>
      ) : null}
      {availability ? (
        <span
          title={availability.label}
          className={cn("ml-auto grid place-items-center", AVAILABILITY_TONE[availability.tone])}
        >
          <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
          <span className="sr-only">{availability.label}</span>
        </span>
      ) : null}
    </div>
  );
}

export interface ModelListProps {
  listId: string;
  /** Stable DOM id for the option at a keyboard row index (active-descendant). */
  optionId: (rowIndex: number) => string;
  listModel: RuntimeListModel;
  highlightIndex: number;
  loading?: boolean;
  providerName: (providerId: string) => string;
  providerKind: (providerId: string) => string;
  onSelect: (provider: string, id: string) => void;
  /** Pointer hover activates a row by its keyboard index. */
  onHover: (rowIndex: number) => void;
  /** Pointer path for a row's favorite star (keyboard path is Alt+F in search). */
  onToggleFavorite: (row: RuntimeGroupModel) => void;
  onCustomCommit: (id: string) => void;
  onFocusSearch: () => void;
}

export function ModelList({
  listId,
  optionId,
  listModel,
  highlightIndex,
  loading = false,
  providerName,
  providerKind,
  onSelect,
  onHover,
  onToggleFavorite,
  onCustomCommit,
  onFocusSearch,
}: ModelListProps) {
  const renderRow = (row: RuntimeGroupModel, showGlyph: boolean) => (
    <ModelRow
      key={row.cursor}
      id={optionId(row.rowIndex)}
      model={row.model}
      providerName={providerName(row.model.provider)}
      iconKind={providerKind(row.model.provider)}
      showGlyph={showGlyph}
      selected={row.selected}
      favorite={row.favorite}
      highlighted={row.rowIndex === highlightIndex}
      onSelect={onSelect}
      onHover={() => onHover(row.rowIndex)}
      onToggleFavorite={() => onToggleFavorite(row)}
    />
  );

  const hasOptions = listModel.flatRows.length > 0;
  const showsOptions = !loading && hasOptions;

  return (
    // Outer scroller owns the overflow so sticky group heads stick to its top,
    // while the `role="listbox"` stays PURE options — the custom-ID affordance is an
    // action, not a selectable option, so it sits OUTSIDE the listbox.
    <div className="flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto px-1 pt-0.5 pb-1.5">
      <div
        role={showsOptions ? "listbox" : "status"}
        id={listId}
        aria-label={showsOptions ? "Models" : undefined}
        aria-live={showsOptions ? undefined : "polite"}
        data-testid="runtime-selector-list"
      >
        {loading ? (
          <SkeletonRows
            aria-label="Loading models"
            className="px-2 py-2"
            count={5}
            data-testid="runtime-selector-loading"
            rowClassName="border-b border-line-soft px-2 py-2.5"
          />
        ) : !hasOptions ? (
          <div
            className="px-4 py-8 text-center text-small-body text-subtle"
            data-testid="runtime-selector-empty"
          >
            No models match your search.
            <br />
            Try a provider name, a shorter query, or an exact model ID.
          </div>
        ) : (
          <>
            {listModel.pinned.length > 0 ? (
              <div>
                <GroupHead name={listModel.pinnedHeading} />
                {/* Pinned rows span providers, so they keep the provider mark. */}
                {listModel.pinned.map(row => renderRow(row, true))}
              </div>
            ) : null}
            {listModel.groups.map(group => (
              <div key={group.key}>
                <GroupHead
                  name={group.name}
                  harnessBadge={group.harnessBadge}
                  availability={group.availability}
                />
                {group.models.map(row => renderRow(row, false))}
              </div>
            ))}
          </>
        )}
      </div>
      {!loading && listModel.customLabel ? (
        <button
          type="button"
          data-testid="runtime-selector-custom"
          className="mx-1 mt-1 mb-0.5 flex w-[calc(100%-0.5rem)] items-center gap-2 rounded-md border border-dashed border-line-strong px-2 py-1.5 text-left text-small-body text-muted outline-none transition-colors hover:border-accent-dim hover:text-fg-strong focus-visible:border-accent-dim focus-visible:text-fg-strong focus-visible:ring-2 focus-visible:ring-accent"
          onClick={() => {
            if (listModel.customCommit) onCustomCommit(listModel.customCommit);
            else onFocusSearch();
          }}
        >
          <Plus aria-hidden="true" className="size-3 shrink-0" />
          <span className="min-w-0 truncate">{listModel.customLabel}</span>
        </button>
      ) : null}
    </div>
  );
}
