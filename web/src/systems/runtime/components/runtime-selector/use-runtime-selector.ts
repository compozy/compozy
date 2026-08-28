import { useSelector, useStore } from "@xstate/store-react";

import type { RuntimeSpeed } from "@/lib/api-contract";

import { runtimeModelKey } from "./model-key";
import { buildRuntimeListModel, type RailFilter } from "./runtime-list-model";
import {
  advancedRuntimeOptions,
  sanitizeRuntimeACPSelections,
  setRuntimeACPSelection,
} from "./runtime-advanced-options-model";
import { runtimeSelectorPopupLogic } from "./runtime-selector-store";
import { useRuntimeFavorites } from "./use-runtime-favorites";
import {
  modelSupportsFast,
  modelSupportsReasoningEffort,
  resolveReasoningState,
  type RuntimeACPOption,
  type RuntimeACPOptionSelection,
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
  onChange: (next: RuntimeSelectorValue, normalizedSpeed?: RuntimeSpeed) => void;
  providers: RuntimeProviderOption[];
  models: RuntimeModelOption[];
  acpOptions?: RuntimeACPOption[];
  allowCustomProvider?: boolean;
  speed?: RuntimeSpeed;
}

function isProviderManaged(provider: RuntimeProviderOption | undefined): boolean {
  return provider?.runtime_strategy === "provider_managed";
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
  acpOptions,
  allowCustomProvider = false,
  speed,
}: UseRuntimeSelectorArgs) {
  const popupStore = useStore(runtimeSelectorPopupLogic);
  const popupState = useSelector(popupStore, snapshot => snapshot.context);
  const open = popupState.phase === "open";
  const railFilter = popupState.phase === "open" ? popupState.railFilter : "all";
  const query = popupState.phase === "open" ? popupState.query : "";
  const entryMode = popupState.phase === "open" ? popupState.entryMode : "catalog";
  const advancedExpanded = popupState.phase === "open" ? popupState.advancedExpanded : false;
  // The active row is a compound (provider,model) cursor, never a list index.
  // The derived index below therefore follows the model through reordering.
  const activeRow = popupState.phase === "open" ? popupState.activeRow : null;
  const favoriteAnnouncement = popupState.phase === "open" ? popupState.favoriteAnnouncement : "";

  const providerById = new Map(providers.map(provider => [provider.id, provider]));
  const modelByKey = new Map(
    models.map(model => [runtimeModelKey(model.provider, model.id), model])
  );
  // Each selector projects shared preferences through its own catalog. Foreign
  // entries stay persisted for other selector surfaces but are never rendered
  // or accepted as mutations here.
  const validKeys = new Set(modelByKey.keys());
  const favorites = useRuntimeFavorites(validKeys);
  const selectedModel = value.model
    ? modelByKey.get(runtimeModelKey(value.provider, value.model))
    : undefined;
  const activeProvider = providerById.get(value.provider);
  const reasoningState = resolveReasoningState(selectedModel);
  const effectiveACPOptions = acpOptions ?? selectedModel?.acp_options;
  const advancedOptions = advancedRuntimeOptions(effectiveACPOptions);
  const sanitizedACPSelections = sanitizeRuntimeACPSelections(
    value.acp_options,
    effectiveACPOptions
  );
  const normalizedReasoning =
    value.reasoning_effort !== "" &&
    !modelSupportsReasoningEffort(selectedModel, value.reasoning_effort)
      ? ""
      : value.reasoning_effort;
  const normalizedValue: RuntimeSelectorValue = {
    ...value,
    reasoning_effort: normalizedReasoning,
    ...(effectiveACPOptions
      ? sanitizedACPSelections
        ? { acp_options: sanitizedACPSelections }
        : { acp_options: undefined }
      : {}),
  };

  const isFavoriteModel = (model: RuntimeModelOption) =>
    favorites.isFavorite(runtimeModelKey(model.provider, model.id));

  // A custom ID targets exactly one EXPLICIT, KNOWN provider — the rail-filtered
  // provider when one is active, otherwise the current selection's provider ONLY
  // when it is itself a real provider. There is no default substitution: when
  // neither is a known provider this is "", which disables the custom commit
  // entirely (a custom ID with no provider target must never be emitted).
  const catalogCustomProvider = resolveActiveCustomProvider(
    railFilter,
    providerById,
    value.provider
  );
  const activeCustomProvider =
    catalogCustomProvider || (allowCustomProvider ? value.provider.trim() : "");

  const listModel = buildRuntimeListModel({
    exactEntry: entryMode === "exact",
    query,
    railFilter,
    models,
    modelByKey,
    providerById,
    value: normalizedValue,
    activeCustomProvider,
    allowCustomProvider,
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
    popupStore.trigger.popupOpened();
  };

  const close = () => popupStore.trigger.popupClosed();

  const setAdvancedExpanded = (expanded: boolean) => {
    popupStore.trigger.advancedOptionsToggled({ expanded });
  };

  const changeRail = (target: RailFilter) => {
    // The rail is a local list filter only — `all`, `fav`, and provider IDs
    // never mutate the controlled value or clear the current model/effort.
    // Selecting a model row is what adopts that model's provider.
    popupStore.trigger.railChanged({ railFilter: target });
  };

  const changeQuery = (next: string) => {
    popupStore.trigger.queryChanged({ query: next });
  };

  const startExactEntry = () => popupStore.trigger.exactEntryStarted();
  const cancelExactEntry = () => popupStore.trigger.exactEntryCanceled();

  const emitSelection = (provider: string, id: string, model: RuntimeModelOption | undefined) => {
    const rz = resolveReasoningState(model);
    const keepsLevel =
      rz.mode === "levels" &&
      (normalizedValue.reasoning_effort === "" ||
        rz.levels.includes(normalizedValue.reasoning_effort));
    const targetACPOptions = acpOptions ?? model?.acp_options;
    const nextSelections = targetACPOptions
      ? sanitizeRuntimeACPSelections(normalizedValue.acp_options, targetACPOptions)
      : undefined;
    const nextValue: RuntimeSelectorValue = {
      ...normalizedValue,
      provider,
      model: id,
      reasoning_effort: keepsLevel ? normalizedValue.reasoning_effort : "",
    };
    nextValue.acp_options = nextSelections;
    const normalizedSpeed =
      speed === "fast" &&
      (isProviderManaged(providerById.get(provider)) || !modelSupportsFast(model))
        ? "normal"
        : undefined;
    onChange(nextValue, normalizedSpeed);
    favorites.pushRecent(runtimeModelKey(provider, id));
  };

  const pickModel = (provider: string, modelId: string) => {
    const id = modelId.trim();
    if (id.length === 0) return;
    if (isProviderManaged(providerById.get(provider))) return;
    const model = modelByKey.get(runtimeModelKey(provider, id));
    if (model?.disabled) return;
    emitSelection(provider, id, model);
  };

  const pickCustom = (modelId: string) => {
    let id = modelId.trim();
    if (id.length === 0) return false;
    let provider = activeCustomProvider;
    if (provider.length === 0 && allowCustomProvider) {
      const separator = id.indexOf("/");
      if (separator <= 0 || separator === id.length - 1) return false;
      provider = id.slice(0, separator).trim();
      id = id.slice(separator + 1).trim();
    }
    // Fail closed on a missing/unknown provider — a custom ID must never be
    // emitted with an empty or guessed provider (no default substitution).
    if (provider.length === 0 || (!allowCustomProvider && !providerById.has(provider)))
      return false;
    if (isProviderManaged(providerById.get(provider))) return false;
    // A custom ID may coincide with a known row for the active provider; reuse
    // its reasoning profile, otherwise commit it as a provisional exact ID.
    emitSelection(provider, id, modelByKey.get(runtimeModelKey(provider, id)));
    return true;
  };

  const commitCustom = (modelId: string) => {
    if (!pickCustom(modelId)) return false;
    if (entryMode === "exact") popupStore.trigger.exactEntryCommitted();
    return true;
  };

  const setReasoning = (effort: RuntimeSelectorValue["reasoning_effort"]) => {
    if (isProviderManaged(activeProvider)) return;
    if (effort !== "" && !modelSupportsReasoningEffort(selectedModel, effort)) return;
    onChange({ ...normalizedValue, reasoning_effort: effort });
  };

  const setACPOption = (selection: RuntimeACPOptionSelection | null) => {
    if (isProviderManaged(activeProvider)) return;
    const nextSelections = setRuntimeACPSelection(
      normalizedValue.acp_options,
      selection,
      effectiveACPOptions
    );
    onChange({
      ...normalizedValue,
      ...(nextSelections ? { acp_options: nextSelections } : { acp_options: undefined }),
    });
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
    popupStore.trigger.activeRowChanged({ row: { cursor: row.cursor, key: row.key } });
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
    popupStore.trigger.activeRowChanged({ row: { cursor: row.cursor, key: row.key } });
  };

  const commitHighlight = () => {
    if (entryMode === "exact") {
      return commitCustom(query);
    }
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

  // Shared favorite commit for both entry points (pointer star + Alt+F). Focus
  // never sits on a per-row control, so a polite status region speaks the
  // result instead of any aria-pressed flip.
  const toggleFavoriteFor = (model: RuntimeModelOption) => {
    if (model.disabled) return false;
    const wasFavorite = isFavoriteModel(model);
    toggleFavorite(model.provider, model.id);
    const providerName = providerById.get(model.provider)?.name ?? model.provider;
    popupStore.trigger.favoriteAnnounced({
      message: `${wasFavorite ? "Unfavorited" : "Favorited"} ${model.name} from ${providerName}`,
    });
    return true;
  };

  const toggleHighlightedFavorite = () => {
    const row = highlightIndex >= 0 ? listModel.flatRows[highlightIndex]?.model : undefined;
    if (!row) return false;
    return toggleFavoriteFor(row);
  };

  // Pointer hover activates a (selectable) row by its compound key so the
  // external favorite action always targets the row under the cursor — no
  // interactive control nests in a listbox option.
  const highlightRow = (rowIndex: number) => {
    const row = listModel.flatRows[rowIndex];
    if (!row || row.model.disabled) return;
    popupStore.trigger.activeRowChanged({ row: { cursor: row.cursor, key: row.key } });
  };

  return {
    favorites,
    open,
    openPopup,
    close,
    railFilter,
    changeRail,
    query,
    changeQuery,
    entryMode,
    advancedExpanded,
    advancedOptions,
    startExactEntry,
    cancelExactEntry,
    highlightIndex,
    highlightRow,
    favoriteAnnouncement,
    moveHighlight,
    moveHighlightEdge,
    commitHighlight,
    toggleFavoriteFor,
    toggleHighlightedFavorite,
    listModel,
    selectedModel,
    activeProvider,
    reasoningState,
    providerManaged: isProviderManaged(activeProvider),
    speedSupported: modelSupportsFast(selectedModel),
    value: normalizedValue,
    pickModel,
    pickCustom,
    commitCustom,
    setReasoning,
    setACPOption,
    setAdvancedExpanded,
  };
}

export type RuntimeSelectorController = ReturnType<typeof useRuntimeSelector>;
