import { Plus, Star } from "lucide-react";

import { cn, Kbd, SkeletonRows } from "@agh/ui";

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
      className="sticky top-0 z-[1] flex items-center gap-2 bg-canvas-soft px-2.5 pt-[9px] pb-[5px]"
    >
      <span className="text-eyebrow font-semibold text-subtle">{name}</span>
      {harnessBadge ? (
        <span className="rounded-xxs bg-badge-fill px-[5px] py-px font-mono text-micro font-semibold tracking-mono text-faint">
          {harnessBadge.toUpperCase()}
        </span>
      ) : null}
      {availability ? (
        <span
          className={cn(
            "ml-auto inline-flex items-center gap-1.5 text-badge font-medium",
            AVAILABILITY_TONE[availability.tone]
          )}
        >
          <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
          {availability.label}
        </span>
      ) : null}
    </div>
  );
}

/** The active row's identity + favorite state, for the external favorite toggle. */
export interface HighlightedFavorite {
  name: string;
  /** Provider display name — part of the button's stable accessible name. */
  providerName: string;
  favorite: boolean;
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
  /** The active row for the external favorite action (undefined when none). */
  highlightedFavorite?: HighlightedFavorite;
  onToggleHighlightedFavorite: () => void;
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
  highlightedFavorite,
  onToggleHighlightedFavorite,
  onCustomCommit,
  onFocusSearch,
}: ModelListProps) {
  const renderRow = (row: RuntimeGroupModel) => (
    <ModelRow
      key={row.cursor}
      id={optionId(row.rowIndex)}
      model={row.model}
      providerName={providerName(row.model.provider)}
      iconKind={providerKind(row.model.provider)}
      selected={row.selected}
      favorite={row.favorite}
      highlighted={row.rowIndex === highlightIndex}
      onSelect={onSelect}
      onHover={() => onHover(row.rowIndex)}
    />
  );

  // Stable accessible name for the external favorite toggle — it never flips
  // Favorite/Unfavorite; aria-pressed carries the on/off truth.
  const favoriteLabel = highlightedFavorite
    ? `Favorite ${highlightedFavorite.name} from ${highlightedFavorite.providerName}`
    : "Favorite the active model";
  const hasOptions = listModel.flatRows.length > 0;
  const showsOptions = !loading && hasOptions;

  return (
    // Outer scroller owns the overflow so sticky group heads stick to its top,
    // while the `role="listbox"` stays PURE options — the custom-ID affordance is an
    // action, not a selectable option, so it sits OUTSIDE the listbox.
    <div className="flex min-w-0 flex-1 flex-col overflow-y-auto px-1.5 pt-1.5 pb-2">
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
            rowClassName="border-b border-line-soft px-2 py-3"
          />
        ) : !hasOptions ? (
          <div
            className="px-5 py-10 text-center text-small-body text-subtle"
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
                {listModel.pinned.map(renderRow)}
              </div>
            ) : null}
            {listModel.groups.map(group => (
              <div key={group.key}>
                <GroupHead
                  name={group.name}
                  harnessBadge={group.harnessBadge}
                  availability={group.availability}
                />
                {group.models.map(renderRow)}
              </div>
            ))}
          </>
        )}
      </div>
      {!loading ? (
        // A REAL favorite toggle, outside the listbox, acting on the active row.
        // Pointer- and keyboard-activatable (Alt+F). Its accessible name is STABLE
        // ("Favorite <model> from <provider>", provider identity included) and does
        // NOT flip on toggle — favorite truth rides on aria-pressed, the toggle-
        // button pattern. Never a fake span nested inside a listbox option. The
        // visible text equals the accessible name (WCAG 2.5.3 label-in-name).
        <button
          type="button"
          data-testid="runtime-selector-favorite-action"
          data-favorite={highlightedFavorite?.favorite ? "true" : "false"}
          disabled={!highlightedFavorite}
          aria-pressed={highlightedFavorite?.favorite ?? false}
          aria-keyshortcuts={highlightedFavorite ? "Alt+F" : undefined}
          aria-label={favoriteLabel}
          onClick={onToggleHighlightedFavorite}
          className="mx-1.5 mt-1 flex w-[calc(100%-0.75rem)] items-center gap-2 rounded-md px-2.5 py-2 text-left text-small-body text-muted outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:text-fg-strong focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed disabled:opacity-50 data-[favorite=true]:text-warning"
        >
          <Star
            aria-hidden="true"
            className={cn("size-3.5 shrink-0", highlightedFavorite?.favorite && "fill-current")}
          />
          <span className="min-w-0 flex-1 truncate">{favoriteLabel}</span>
          <Kbd className="shrink-0">⌥F</Kbd>
        </button>
      ) : null}
      {!loading && listModel.customLabel ? (
        <button
          type="button"
          data-testid="runtime-selector-custom"
          className="mx-1.5 mt-1 mb-0.5 flex w-[calc(100%-0.75rem)] items-center gap-2.5 rounded-md border border-dashed border-line-strong px-2.5 py-2.5 text-left text-small-body text-muted outline-none transition-colors hover:border-accent-dim hover:text-fg-strong focus-visible:border-accent-dim focus-visible:text-fg-strong focus-visible:ring-2 focus-visible:ring-accent"
          onClick={() => {
            if (listModel.customCommit) onCustomCommit(listModel.customCommit);
            else onFocusSearch();
          }}
        >
          <Plus aria-hidden="true" className="size-3.5" />
          {listModel.customLabel}
        </button>
      ) : null}
    </div>
  );
}
