import { runtimeModelKey } from "./model-key";
import type { RuntimeModelOption, RuntimeProviderOption, RuntimeSelectorValue } from "./types";

export type RailFilter = "all" | "fav" | (string & {});

export interface GroupAvailability {
  tone: "success" | "warning" | "danger";
  label: string;
}

export interface RuntimeGroupModel {
  model: RuntimeModelOption;
  /** Compound `(provider, model)` identity — the DOM/list/favorite/recent key. */
  key: string;
  /** Stable identity for this rendered pinned/group occurrence. */
  cursor: string;
  /** Position among all selectable (model) rows in render order — keyboard index. */
  rowIndex: number;
  selected: boolean;
  favorite: boolean;
}

export interface RuntimeListGroup {
  key: string;
  name: string;
  /** Icon key for the group glyph (provider `runtime_provider` or id). */
  iconKind: string;
  harnessBadge?: string;
  availability: GroupAvailability;
  models: RuntimeGroupModel[];
}

export interface RuntimeListModel {
  searching: boolean;
  pinned: RuntimeGroupModel[];
  pinnedHeading: string;
  groups: RuntimeListGroup[];
  customLabel: string;
  customCommit: string;
  /** Rendered model rows in keyboard order, including pinned/group occurrences. */
  flatRows: RuntimeGroupModel[];
}

interface BuildRuntimeListModelArgs {
  query: string;
  railFilter: RailFilter;
  models: RuntimeModelOption[];
  modelByKey: Map<string, RuntimeModelOption>;
  providerById: Map<string, RuntimeProviderOption>;
  value: RuntimeSelectorValue;
  activeCustomProvider: string;
  recentKeys: string[];
  isFavoriteModel: (model: RuntimeModelOption) => boolean;
}

const HARNESS_BADGES: Record<string, string> = { acp: "cli", pi_acp: "api key" };

function providerBadge(harness: string | undefined): string | undefined {
  const key = (harness ?? "").trim().toLowerCase();
  return HARNESS_BADGES[key];
}

function providerIconKind(provider: RuntimeProviderOption | undefined, providerId: string): string {
  return provider?.runtime_provider ?? provider?.id ?? providerId;
}

function groupAvailability(
  provider: RuntimeProviderOption | undefined,
  models: RuntimeModelOption[]
): GroupAvailability {
  if (provider?.needs_auth) return { tone: "danger", label: "Sign in" };
  if (models.some(model => model.availability === "stale")) {
    return { tone: "warning", label: "Stale" };
  }
  if (models.length > 0 && models.every(model => model.availability === "unavailable")) {
    return { tone: "danger", label: "Unavailable" };
  }
  return { tone: "success", label: "Live" };
}

function matchesQuery(model: RuntimeModelOption, providerName: string, tokens: string[]): boolean {
  if (tokens.length === 0) return true;
  const haystack = `${model.name} ${model.id} ${model.provider} ${providerName}`.toLowerCase();
  return tokens.every(token => haystack.includes(token));
}

/** Ranking within a visible set: favorites first, then featured, then daemon order (stable). */
function rankModels(
  models: RuntimeModelOption[],
  isFavorite: (model: RuntimeModelOption) => boolean
): RuntimeModelOption[] {
  return models
    .map((model, index) => ({ model, index }))
    .sort((a, b) => {
      const favDelta = Number(isFavorite(b.model)) - Number(isFavorite(a.model));
      if (favDelta !== 0) return favDelta;
      const featuredDelta = Number(Boolean(b.model.featured)) - Number(Boolean(a.model.featured));
      if (featuredDelta !== 0) return featuredDelta;
      return a.index - b.index;
    })
    .map(entry => entry.model);
}

export function buildRuntimeListModel({
  query,
  railFilter,
  models,
  modelByKey,
  providerById,
  value,
  activeCustomProvider,
  recentKeys,
  isFavoriteModel,
}: BuildRuntimeListModelArgs): RuntimeListModel {
  const trimmedQuery = query.trim().toLowerCase();
  const tokens = trimmedQuery.length > 0 ? trimmedQuery.split(/\s+/) : [];
  const searching = tokens.length > 0;
  // The rail is a local list filter only; a non-empty search always spans
  // every provider (design HTML: search ignores the rail selection).
  const effectiveRail: RailFilter = searching ? "all" : railFilter;
  const providerDisplayName = (id: string): string => providerById.get(id)?.name ?? id;
  const flatRows: RuntimeGroupModel[] = [];

  const toRow = (model: RuntimeModelOption, occurrence: "pinned" | "group") => {
    const key = runtimeModelKey(model.provider, model.id);
    const row: RuntimeGroupModel = {
      model,
      key,
      cursor: `${occurrence}:${key}`,
      rowIndex: flatRows.length,
      selected: model.provider === value.provider && model.id === value.model,
      favorite: isFavoriteModel(model),
    };
    flatRows.push(row);
    return row;
  };

  // Pinned recents + favorites (browse only, "all"/"fav" rail). Both span
  // providers: favorites and recents are keyed by the compound identity.
  let pinned: RuntimeGroupModel[] = [];
  let pinnedHeading = "";
  if (!searching && (railFilter === "all" || railFilter === "fav")) {
    const favoriteModels = models.filter(isFavoriteModel);
    if (railFilter === "fav") {
      pinned = favoriteModels.map(model => toRow(model, "pinned"));
      pinnedHeading = "Favorites";
    } else {
      const recentModels = recentKeys
        .map(key => modelByKey.get(key))
        .filter((model): model is RuntimeModelOption => model !== undefined);
      const seen = new Set<string>();
      const merged: RuntimeModelOption[] = [];
      for (const model of [...recentModels, ...favoriteModels]) {
        const key = runtimeModelKey(model.provider, model.id);
        if (seen.has(key)) continue;
        seen.add(key);
        merged.push(model);
      }
      pinned = merged.map(model => toRow(model, "pinned"));
      pinnedHeading = "Recent & favorites";
    }
  }

  // Grouped models across providers. Browse shows the curated subset; search
  // spans the full `view=all` set. A provider-scoped rail is a local filter
  // over the loaded cross-provider set; "fav" hides the grouped section.
  const groups: RuntimeListGroup[] = [];
  if (effectiveRail !== "fav") {
    const providerFilter = effectiveRail === "all" ? null : effectiveRail;
    const grouped = new Map<string, RuntimeModelOption[]>();
    for (const model of models) {
      if (providerFilter && model.provider !== providerFilter) continue;
      if (!searching && !model.curated) continue;
      if (searching && !matchesQuery(model, providerDisplayName(model.provider), tokens)) continue;
      const bucket = grouped.get(model.provider) ?? [];
      bucket.push(model);
      grouped.set(model.provider, bucket);
    }
    for (const [providerId, bucket] of grouped) {
      const provider = providerById.get(providerId);
      groups.push({
        key: providerId,
        name: provider?.name ?? providerId,
        iconKind: providerIconKind(provider, providerId),
        harnessBadge: providerBadge(provider?.harness),
        availability: groupAvailability(provider, bucket),
        models: rankModels(bucket, isFavoriteModel).map(model => toRow(model, "group")),
      });
    }
  }

  // A custom ID requires an explicit provider target. Known-match is scoped to
  // the exact (activeProvider, id) tuple, and model ids remain case-sensitive.
  const customId = query.trim();
  const knownForActiveProvider =
    activeCustomProvider.length > 0 &&
    modelByKey.has(runtimeModelKey(activeCustomProvider, customId));
  const canCommitCustom = searching && activeCustomProvider.length > 0 && !knownForActiveProvider;
  const customCommit = canCommitCustom ? customId : "";
  const customLabel = customCommit ? `Use "${customCommit}"` : "Use an exact custom model ID…";
  const showCustom = effectiveRail !== "fav" && activeCustomProvider.length > 0;

  return {
    searching,
    pinned,
    pinnedHeading,
    groups,
    customLabel: showCustom ? customLabel : "",
    customCommit,
    flatRows,
  };
}
