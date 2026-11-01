import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import type { Filter, ListingViewMode } from "@compozy/ui";

import { normalizeListingSearchValue } from "@/lib/listing-search";
import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";
import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import {
  AGENT_PLACE_LATER,
  GLOBAL_SCOPE,
  agentTeamOptions,
  buildAgentTeam,
  buildAgentTeamFilterFields,
  hasActiveAgentFleetFilters,
  matchesAgentFilters,
  matchesAgentFocus,
  useAgentCreateHost,
  useAgentInstanceActions,
  useAgentInstanceCapacity,
  useAgentInstances,
  type AgentAddPlaceActions,
  type AgentInstanceCapacity,
  type AgentTeamFocus,
  type AgentTeamMember,
  type AgentsFleetSearch,
} from "@/systems/agent";
import { useProfileReadScope } from "@/systems/profiles";
import {
  useSessionCreateActions,
  useSessionCreateHasActiveWorkspace,
  useSignedOutProviders,
} from "@/systems/session";
import {
  focusedWorkspaceSelector,
  ownerDataScope,
  useActiveWorkspace,
  useWindowDataWorkspace,
} from "@/systems/workspace";
import { useOpenTarget } from "../../hooks/use-open-target";

const SEARCH_DEBOUNCE_MS = 200;

const FOCUS_ITEMS: readonly { value: AgentTeamFocus; label: string }[] = [
  { value: "all", label: "Everyone" },
  { value: "needs-you", label: "Needs you" },
  { value: "working", label: "Working" },
  { value: "paused", label: "Paused" },
];

const CHIP_FIELDS = ["project", "model", "owner"] as const;

function searchChips(search: AgentsFleetSearch): Filter<string>[] {
  return CHIP_FIELDS.flatMap(field => {
    const value = search[field];
    return value
      ? [{ id: `agent-team-filter-${field}`, field, operator: "is", values: [value] }]
      : [];
  });
}

function chipValue(chips: readonly Filter<string>[], field: string) {
  return chips.find(chip => chip.field === field)?.values[0];
}

function useTeamMembers(enabled: boolean) {
  const { workspaces } = useActiveWorkspace();
  const { destination } = useProfileReadScope();
  const dataScope = useWindowDataWorkspace();
  // All projects reads every project's definitions; a focused window reads its own.
  const scope = dataScope?.kind === "all" ? "" : focusedWorkspaceSelector(dataScope);
  const team = useQuery(agentTeamOptions(scope, destination, enabled));
  const instances = useAgentInstances();
  const inWindow =
    dataScope?.kind === "project"
      ? instances.filter(
          instance => instance.workspace_id === dataScope.workspaceId || !instance.workspace_id
        )
      : instances;
  const members = buildAgentTeam({
    items: team.data ?? [],
    instances: inWindow,
    workspaces,
  });
  return { team, members };
}

function useAddPlaceActions(capacity: AgentInstanceCapacity): AgentAddPlaceActions {
  const navigate = useNavigate();
  const { workspaces } = useActiveWorkspace();
  const actions = useAgentInstanceActions();
  const signedOut = useSignedOutProviders();
  return {
    places: [{ id: GLOBAL_SCOPE, name: "Global" }, ...workspaces],
    capacity,
    signedOut,
    onAdd: (agent, place) => actions.add({ agent, workspace: place }),
    onSignIn: () => void navigate({ to: "/settings/providers" }),
  };
}

/** All profiles reads definitions for the acting profile only (configuration resolves per profile). */
function useTeamDefinitionsProfile() {
  const profile = useProfileReadScope();
  return profile.aggregate ? profile.destination : null;
}

function fleetSearchFacets(search: AgentsFleetSearch) {
  const focus: AgentTeamFocus = search.focus ?? "all";
  const view: ListingViewMode = search.view ?? "cards";
  return { chips: searchChips(search), focus, view };
}

function fleetListingState({
  isLoading,
  isError,
  total,
  shown,
  unfiltered,
}: {
  isLoading: boolean;
  isError: boolean;
  total: number;
  shown: number;
  unfiltered: boolean;
}) {
  const settled = !isLoading && !isError;
  const isFirstRunEmpty = settled && total === 0 && unfiltered;
  return {
    isLoading,
    isFirstRunEmpty,
    isFilteredEmpty: settled && shown === 0 && !isFirstRunEmpty,
    showFacets: !isLoading && !isFirstRunEmpty && !(isError && total === 0),
  };
}

function useOpenTeamMember(allProjects: boolean) {
  const openItem = useOpenTarget();
  const navigate = useNavigate({ from: "/agents" });
  return (member: AgentTeamMember) => {
    const route = { pathname: `/agents/${encodeURIComponent(member.name)}`, search: {} };
    if (allProjects) {
      // All projects: an agent opens in place, its window bound to the agent's project.
      openItem({ app: "agents", binding: ownerDataScope(member.agent?.workspace_id), route });
      return;
    }
    void navigate({ to: "/agents/$name", params: { name: member.name } });
  };
}

function useOpenAgentCreate() {
  const { openDialog } = useAgentCreateHost();
  return (options?: { staysOn?: boolean }) =>
    openDialog(options?.staysOn ? { place: AGENT_PLACE_LATER } : undefined);
}

function focusItemsFor(members: readonly AgentTeamMember[]) {
  return FOCUS_ITEMS.flatMap(item => {
    if (item.value === "all") return [item];
    const count = members.filter(member => matchesAgentFocus(member, item.value)).length;
    return count > 0 ? [{ ...item, badge: count }] : [];
  });
}

function providerOptionsOf(members: readonly AgentTeamMember[]) {
  return [...new Set(members.flatMap(member => member.provider ?? []))].map(id => ({
    id,
    name: id.charAt(0).toUpperCase() + id.slice(1),
  }));
}

const withChips = (next: readonly Filter<string>[]) => (current: AgentsFleetSearch) => ({
  ...current,
  project: chipValue(next, "project"),
  model: chipValue(next, "model"),
  owner: chipValue(next, "owner"),
});

const withFocus = (next: AgentTeamFocus) => (current: AgentsFleetSearch) => ({
  ...current,
  focus: next === "all" ? undefined : next,
});

const withView = (next: ListingViewMode) => (current: AgentsFleetSearch) => ({
  ...current,
  view: next === "cards" ? undefined : next,
});

function useAgentsFleetPage(search: AgentsFleetSearch = {}) {
  const { pending, workspaces } = useActiveWorkspace();
  const definitionsFor = useTeamDefinitionsProfile();
  const dataScope = useWindowDataWorkspace();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const navigate = useNavigate({ from: "/agents" });
  const onOpen = useOpenTeamMember(dataScope?.kind === "all");
  const openCreate = useOpenAgentCreate();
  const { openForAgent } = useSessionCreateActions();
  const hasActiveWorkspace = useSessionCreateHasActiveWorkspace();
  const searchInputRef = useListingSearchShortcut(liveDataEnabled);
  const updateSearch = (updater: (current: AgentsFleetSearch) => AgentsFleetSearch) => {
    void navigate({ search: current => updater(current), to: "/agents" });
  };
  const queryInput = useDebouncedInput({
    delayMs: SEARCH_DEBOUNCE_MS,
    externalValue: search.q ?? "",
    onCommit: nextQuery =>
      updateSearch(current => ({ ...current, q: normalizeListingSearchValue(nextQuery) })),
  });

  const { team, members } = useTeamMembers(liveDataEnabled && !pending && dataScope !== null);
  const capacity = useAgentInstanceCapacity();
  const add = useAddPlaceActions(capacity);
  const { chips, focus, view } = fleetSearchFacets(search);

  const matched = members.filter(member =>
    matchesAgentFilters(member, queryInput.draftValue, chips, workspaces)
  );
  const shown = matched.filter(member => matchesAgentFocus(member, focus));
  const focusItems = focusItemsFor(members);
  const providers = providerOptionsOf(members);

  const unfiltered = !hasActiveAgentFleetFilters(search) && queryInput.draftValue === "";
  const listing = fleetListingState({
    isLoading: pending || team.isPending,
    isError: team.isError,
    total: members.length,
    shown: shown.length,
    unfiltered,
  });

  return {
    members: shown,
    total: members.length,
    view,
    chips,
    focus,
    focusItems,
    fields: buildAgentTeamFilterFields(workspaces, providers),
    draftQuery: queryInput.draftValue,
    searchInputRef,
    unfiltered,
    capacity,
    ...listing,
    agentsError: team.error,
    definitionsFor,
    setDraftQuery: queryInput.setDraftValue,
    setChips: (next: readonly Filter<string>[]) => updateSearch(withChips(next)),
    setFocus: (next: AgentTeamFocus) => updateSearch(withFocus(next)),
    setView: (next: ListingViewMode) => updateSearch(withView(next)),
    clearFilters: () => {
      queryInput.reset("");
      updateSearch(current => ({ view: current.view }));
    },
    openCreate,
    actions: {
      add,
      newSessionDisabled: !hasActiveWorkspace,
      onNewSession: (name: string) => openForAgent(name),
      onOpen,
    },
    retryAgents: () => {
      void team.refetch();
    },
  };
}

export { useAgentsFleetPage };
export type { AgentsFleetSearch };
