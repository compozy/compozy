import { useState } from "react";

import { runtimeModelKey } from "./model-key";
import { buildRuntimeListModel, type RailFilter } from "./runtime-list-model";
import { useRuntimeFavorites } from "./use-runtime-favorites";
import {
  resolveReasoningState,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "./types";

export type {
  GroupAvailability,
  RailFilter,
  RuntimeGroupModel,
  RuntimeListGroup,
  RuntimeListModel,
} from "./runtime-list-model";

export interface UseRuntimeSelectorArgs {
  value: RuntimeSelectorValue;
  onChange: (next: RuntimeSelectorValue) => void;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  /**
   * The catalog query has RESOLVED (not merely "not loading"). Drives the strict
   * favorites/recents purge so a legitimately loaded-empty catalog wipes stale
   * persisted entries, while a still-loading/disabled query never does.
   */
  catalogLoaded?: boolean;
}

function resolveActiveCustomProvider(
  railFilter: RailFilter,
  providerById: Map<string, RuntimeProviderOption>,
  selectedProvider: string
): string {
  if (railFilter !== "all" && railFilter !== "fav" && providerById.has(railFilter)) {
    return railFilter;
  }
  return providerById.has(selectedProvider) ? selectedProvider : "";
}

export function useRuntimeSelector({
  value,
  onChange,
  providers,
  models,
  catalogLoaded = false,
}: UseRuntimeSelectorArgs) {
  const [open, setOpen] = useState(false);
  const [railFilter, setRailFilter] = useState<RailFilter>("all");
  const [query, setQuery] = useState("");
  // The active (keyboard/pointer) row is tracked by its compound (provider,model)
  // KEY, never a numeric list index. Favorite-driven reordering rebuilds the list
  // but keeps the same active target — the derived index below just follows it.
  const [activeRow, setActiveRow] = useState<{ cursor: string; key: string } | null>(null);
  // Polite announcement text for the favorite toggle (Alt+F while focus stays in
  // search): "Favorited …" / "Unfavorited …" spoken via an aria-live region.
  const [favoriteAnnouncement, setFavoriteAnnouncement] = useState("");

  const providerById = new Map(providers.map(provider => [provider.id, provider]));
  const modelByKey = new Map(
    models.map(model => [runtimeModelKey(model.provider, model.id), model])
  );
  // Favorites/recents are validated against the current compound-tuple keys so
  // stale/foreign entries are never shown or persisted (see use-runtime-favorites).
  const validKeys = new Set(modelByKey.keys());
  const favorites = useRuntimeFavorites(validKeys, catalogLoaded);
  const selectedModel = value.model
    ? modelByKey.get(runtimeModelKey(value.provider, value.model))
    : undefined;
  const activeProvider = providerById.get(value.provider);
  const reasoningState = resolveReasoningState(selectedModel);

  const isFavoriteModel = (model: RuntimeModelOption) =>
    favorites.isFavorite(runtimeModelKey(model.provider, model.id));

  // A custom ID targets exactly one EXPLICIT, KNOWN provider — the rail-filtered
  // provider when one is active, otherwise the current selection's provider ONLY
  // when it is itself a real provider. There is no default substitution: when
  // neither is a known provider this is "", which disables the custom commit
  // entirely (a custom ID with no provider target must never be emitted).
  const activeCustomProvider = resolveActiveCustomProvider(
    railFilter,
    providerById,
    value.provider
  );

  const listModel = buildRuntimeListModel({
    query,
    railFilter,
    models,
    modelByKey,
    providerById,
    value,
    activeCustomProvider,
    recentKeys: favorites.recents,
    isFavoriteModel,
  });

  // Numeric position of the active row, DERIVED from its stable compound key
  // against the current render order. When favoriting reorders the list the key
  // is unchanged, so this index (and the live `aria-activedescendant`) tracks the
  // same (provider, model) target to its new position instead of pointing at
  // whatever model happens to land on the old index.
  let highlightIndex = -1;
  if (activeRow) {
    const exact = listModel.flatRows.findIndex(row => row.cursor === activeRow.cursor);
    highlightIndex =
      exact >= 0 ? exact : listModel.flatRows.findIndex(row => row.key === activeRow.key);
  }

  const openPopup = () => {
    setRailFilter("all");
    setQuery("");
    setActiveRow(null);
    setFavoriteAnnouncement("");
    setOpen(true);
  };

  const close = () => setOpen(false);

  const changeRail = (target: RailFilter) => {
    // The rail is a local list filter only — `all`, `fav`, and provider IDs
    // never mutate the controlled value or clear the current model/effort.
    // Selecting a model row is what adopts that model's provider.
    setActiveRow(null);
    setRailFilter(target);
  };

  const changeQuery = (next: string) => {
    setQuery(next);
    setActiveRow(null);
  };

  const emitSelection = (provider: string, id: string, model: RuntimeModelOption | undefined) => {
    const rz = resolveReasoningState(model);
    const keepsLevel =
      rz.mode === "levels" &&
      (value.reasoning_effort === "" || rz.levels.includes(value.reasoning_effort));
    onChange({
      provider,
      model: id,
      reasoning_effort: keepsLevel ? value.reasoning_effort : "",
    });
    favorites.pushRecent(runtimeModelKey(provider, id));
  };

  const pickModel = (provider: string, modelId: string) => {
    const id = modelId.trim();
    if (id.length === 0) return;
    const model = modelByKey.get(runtimeModelKey(provider, id));
    if (model?.disabled) return;
    emitSelection(provider, id, model);
  };

  const pickCustom = (modelId: string) => {
    const id = modelId.trim();
    if (id.length === 0) return;
    const provider = activeCustomProvider;
    // Fail closed on a missing/unknown provider — a custom ID must never be
    // emitted with an empty or guessed provider (no default substitution).
    if (provider.length === 0 || !providerById.has(provider)) return;
    // A custom ID may coincide with a known row for the active provider; reuse
    // its reasoning profile, otherwise commit it as a provisional exact ID.
    emitSelection(provider, id, modelByKey.get(runtimeModelKey(provider, id)));
  };

  const setReasoning = (effort: RuntimeSelectorValue["reasoning_effort"]) => {
    onChange({ ...value, reasoning_effort: effort });
  };

  const toggleFavorite = (provider: string, id: string) =>
    favorites.toggleFavorite(runtimeModelKey(provider, id));

  // Keyboard moves resolve to a target row index, then commit the row's compound
  // KEY as the active target (never a raw index) so the highlight survives any
  // later reorder of the same list.
  const moveHighlight = (direction: 1 | -1) => {
    const rows = listModel.flatRows;
    const enabled: number[] = [];
    rows.forEach((row, index) => {
      if (!row.model.disabled) enabled.push(index);
    });
    if (enabled.length === 0) return;
    let target: number;
    if (highlightIndex < 0) {
      target = direction === 1 ? enabled[0] : enabled[enabled.length - 1];
    } else {
      const position = enabled.indexOf(highlightIndex);
      target =
        position < 0
          ? enabled[0]
          : enabled[(position + direction + enabled.length) % enabled.length];
    }
    const row = rows[target];
    setActiveRow({ cursor: row.cursor, key: row.key });
  };

  const moveHighlightEdge = (edge: "first" | "last") => {
    const rows = listModel.flatRows;
    const enabled: number[] = [];
    rows.forEach((row, index) => {
      if (!row.model.disabled) enabled.push(index);
    });
    if (enabled.length === 0) return;
    const target = edge === "first" ? enabled[0] : enabled[enabled.length - 1];
    const row = rows[target];
    setActiveRow({ cursor: row.cursor, key: row.key });
  };

  const commitHighlight = () => {
    const row = highlightIndex >= 0 ? listModel.flatRows[highlightIndex]?.model : undefined;
    if (row && !row.disabled) {
      pickModel(row.provider, row.id);
      return true;
    }
    if (listModel.customCommit.length > 0) {
      pickCustom(listModel.customCommit);
      return true;
    }
    return false;
  };

  const toggleHighlightedFavorite = () => {
    const row = highlightIndex >= 0 ? listModel.flatRows[highlightIndex]?.model : undefined;
    if (!row || row.disabled) return false;
    const wasFavorite = isFavoriteModel(row);
    toggleFavorite(row.provider, row.id);
    // Focus stays in the search field during Alt+F, so the button's aria-pressed
    // flip is not announced; a polite status carries the result instead.
    const providerName = providerById.get(row.provider)?.name ?? row.provider;
    setFavoriteAnnouncement(
      `${wasFavorite ? "Unfavorited" : "Favorited"} ${row.name} from ${providerName}`
    );
    return true;
  };

  // Pointer hover activates a (selectable) row by its compound key so the
  // external favorite action always targets the row under the cursor — no
  // interactive control nests in a listbox option.
  const highlightRow = (rowIndex: number) => {
    const row = listModel.flatRows[rowIndex];
    if (!row || row.model.disabled) return;
    setActiveRow({ cursor: row.cursor, key: row.key });
  };

  // The currently-highlighted model + its favorite state, for the external
  // favorite toggle button's label/pressed state (undefined when none is active).
  const highlightedModel =
    highlightIndex >= 0 ? listModel.flatRows[highlightIndex]?.model : undefined;
  const highlightedRow =
    highlightedModel && !highlightedModel.disabled
      ? { model: highlightedModel, favorite: isFavoriteModel(highlightedModel) }
      : undefined;

  return {
    favorites,
    open,
    openPopup,
    close,
    railFilter,
    changeRail,
    query,
    changeQuery,
    highlightIndex,
    highlightRow,
    highlightedRow,
    favoriteAnnouncement,
    moveHighlight,
    moveHighlightEdge,
    commitHighlight,
    toggleHighlightedFavorite,
    listModel,
    selectedModel,
    activeProvider,
    reasoningState,
    pickModel,
    pickCustom,
    setReasoning,
  };
}

export type RuntimeSelectorController = ReturnType<typeof useRuntimeSelector>;
