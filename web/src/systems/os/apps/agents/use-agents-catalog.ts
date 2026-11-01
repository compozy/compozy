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
  agentInstanceLimitReached,
  agentTeamOptions,
  buildAgentTeam,
  buildAgentTeamFilterFields,
  hasActiveAgentFleetFilters,
  matchesAgentFilters,
  matchesAgentFocus,
  useAgentCreateHost,
  useAgentInstanceActions,
  useAgentInstances,
  type AgentAddPlaceActions,
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
  return { team, instances, members };
}

function useAddPlaceActions(instancesCapped: boolean): AgentAddPlaceActions {
  const navigate = useNavigate();
  const { workspaces } = useActiveWorkspace();
  const actions = useAgentInstanceActions();
  const signedOut = useSignedOutProviders();
  return {
    places: [{ id: GLOBAL_SCOPE, name: "Global" }, ...workspaces],
    capped: instancesCapped,
    signedOut,
    onAdd: (agent, place) => actions.add({ agent, workspace: place }),
    onSignIn: () => void navigate({ to: "/settings/providers" }),
  };
}

function useAgentsFleetPage(search: AgentsFleetSearch = {}) {
  const { pending, workspaces } = useActiveWorkspace();
  const profile = useProfileReadScope();
  const openItem = useOpenTarget();
  const dataScope = useWindowDataWorkspace();
  const allProjects = dataScope?.kind === "all";
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const navigate = useNavigate({ from: "/agents" });
  const { openDialog } = useAgentCreateHost();
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

  const { team, instances, members } = useTeamMembers(
    liveDataEnabled && !pending && dataScope !== null
  );
  const capped = agentInstanceLimitReached(instances);
  const add = useAddPlaceActions(capped);
  const chips = searchChips(search);
  const focus: AgentTeamFocus = search.focus ?? "all";
  const view: ListingViewMode = search.view ?? "cards";

  const matched = members.filter(member =>
    matchesAgentFilters(member, queryInput.draftValue, chips, workspaces)
  );
  const shown = matched.filter(member => matchesAgentFocus(member, focus));
  const focusItems = FOCUS_ITEMS.flatMap(item => {
    if (item.value === "all") return [item];
    const count = members.filter(member => matchesAgentFocus(member, item.value)).length;
    return count > 0 ? [{ ...item, badge: count }] : [];
  });
  const providers = [...new Set(members.flatMap(member => member.provider ?? []))].map(id => ({
    id,
    name: id.charAt(0).toUpperCase() + id.slice(1),
  }));

  const isLoading = pending || team.isPending;
  const unfiltered = !hasActiveAgentFleetFilters(search) && queryInput.draftValue === "";
  const isFirstRunEmpty = !isLoading && !team.isError && members.length === 0 && unfiltered;
  const isFilteredEmpty = !isLoading && !team.isError && shown.length === 0 && !isFirstRunEmpty;

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
    capped,
    isLoading,
    isFirstRunEmpty,
    isFilteredEmpty,
    agentsError: team.error,
    showFacets: !isLoading && !isFirstRunEmpty && !(team.isError && members.length === 0),
    /** All profiles reads definitions for the acting profile only (configuration resolves per profile). */
    definitionsFor: profile.aggregate ? profile.destination : null,
    setDraftQuery: queryInput.setDraftValue,
    setChips: (next: readonly Filter<string>[]) =>
      updateSearch(current => ({
        ...current,
        project: chipValue(next, "project"),
        model: chipValue(next, "model"),
        owner: chipValue(next, "owner"),
      })),
    setFocus: (next: AgentTeamFocus) =>
      updateSearch(current => ({ ...current, focus: next === "all" ? undefined : next })),
    setView: (next: ListingViewMode) =>
      updateSearch(current => ({ ...current, view: next === "cards" ? undefined : next })),
    clearFilters: () => {
      queryInput.reset("");
      updateSearch(current => ({ view: current.view }));
    },
    openCreate: (options?: { staysOn?: boolean }) =>
      openDialog(options?.staysOn ? { place: AGENT_PLACE_LATER } : undefined),
    actions: {
      add,
      newSessionDisabled: !hasActiveWorkspace,
      onNewSession: (name: string) => openForAgent(name),
      onOpen: (member: AgentTeamMember) => {
        const route = { pathname: `/agents/${encodeURIComponent(member.name)}`, search: {} };
        if (allProjects) {
          // All projects: an agent opens in place, its window bound to the agent's project.
          openItem({ app: "agents", binding: ownerDataScope(member.agent?.workspace_id), route });
          return;
        }
        void navigate({ to: "/agents/$name", params: { name: member.name } });
      },
    },
    retryAgents: () => {
      void team.refetch();
    },
  };
}

export { useAgentsFleetPage };
export type { AgentsFleetSearch };
