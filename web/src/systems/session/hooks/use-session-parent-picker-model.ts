import { useQuery } from "@tanstack/react-query";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useProfileReadScope } from "@/systems/profiles";
import { ownerDataScope } from "@/systems/workspace";

import type { SessionParentPickerProps } from "../components/session-parent-picker";
import { sessionScopedDetailOptions } from "../lib/query-options";
import { sessionMoveUnderRefusal, sessionParentCandidates } from "../lib/session-reparent";
import type { SessionPayload } from "../types";
import { useSessionCatalog } from "./use-session-catalog";
import { closeSessionParentPicker } from "./use-session-parent-picker";
import type { useSessionReparent } from "./use-session-reparent";

const PARENT_PAGE_SIZE = 50;

export type SessionParentPickerModel = Omit<SessionParentPickerProps, "open">;

/**
 * Owns Move under… for one session: the source read, the parent catalog, its
 * search and pages, and the moves. `reparent` comes from the always-mounted
 * host so a move's toast and Undo outlive the picker.
 */
export function useSessionParentPickerModel(
  sessionId: string,
  reparent: ReturnType<typeof useSessionReparent>
): SessionParentPickerModel {
  const { params } = useProfileReadScope();
  // The session may belong to another profile (a window kept open under its owner
  // banner); read it across profiles so every move names its own owner.
  const sourceQuery = useQuery(
    sessionScopedDetailOptions(sessionId, { all_profiles: true }, { liveTail: false })
  );
  const source = sourceQuery.data ?? null;
  const {
    draftValue: search,
    committedValue: remoteSearch,
    setDraftValue: setSearch,
  } = useDebouncedInput({ externalValue: "", onCommit: () => undefined });
  // The daemon adopts only an unarchived user session of the same project and
  // profile that is not in Later and not in the session's own tree; the catalog
  // applies every one of those scopes before it cuts a page.
  const catalog = useSessionCatalog(
    source ? ownerDataScope(source.workspace_id) : null,
    {
      limit: PARENT_PAGE_SIZE,
      q: remoteSearch,
      search_fields: "title_agent",
      type: "user",
      subagents: "exclude",
      archive: "exclude",
      later: "exclude",
      exclude_root: sessionId,
    },
    source !== null && sessionMoveUnderRefusal(source) === null,
    { facets: false, lens: source ? { profile: source.profile_name } : params }
  );
  const committed = search === remoteSearch;
  return {
    source,
    sourceFailed: sourceQuery.isError,
    candidates: source ? sessionParentCandidates(source, catalog.sessions) : [],
    search,
    onSearchChange: setSearch,
    loading: catalog.loading,
    // Rows of a previous search or page stay on screen while the next one loads;
    // they must not be chosen by an Enter meant for the new query.
    stale: catalog.paging || !committed,
    failed: catalog.failed,
    onRetry: catalog.retry,
    hasNext: committed && catalog.next,
    hasPrevious: committed && catalog.previous,
    onNextPage: catalog.nextPage,
    onPreviousPage: catalog.previousPage,
    pending: reparent.pending,
    onChoose: (parent: SessionPayload) => {
      if (!source) return;
      reparent.adopt(source, parent);
      closeSessionParentPicker();
    },
    onMakeTopLevel: () => {
      if (!source) return;
      reparent.detach(source);
      closeSessionParentPicker();
    },
    onOpenChange: open => {
      if (!open) closeSessionParentPicker();
    },
  };
}
