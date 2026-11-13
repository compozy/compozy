import { useQuery } from "@tanstack/react-query";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useProfileReadScope } from "@/systems/profiles";
import { ownerDataScope } from "@/systems/workspace";

import { useSessionCatalog } from "../hooks/use-session-catalog";
import {
  closeSessionParentPicker,
  useSessionParentPickerRequest,
} from "../hooks/use-session-parent-picker";
import { useSessionReparent } from "../hooks/use-session-reparent";
import { sessionScopedDetailOptions } from "../lib/query-options";
import { sessionMoveUnderRefusal, sessionParentCandidates } from "../lib/session-reparent";
import type { SessionPayload } from "../types";
import { SessionParentPicker } from "./session-parent-picker";

const PARENT_PAGE_SIZE = 50;

type SessionReparent = ReturnType<typeof useSessionReparent>;

/**
 * The shell's one Move under… picker. The mutation lives here, outside the
 * per-request body, so its toast and Undo still land after the picker closes.
 */
export function SessionParentPickerHost() {
  const sessionId = useSessionParentPickerRequest();
  const reparent = useSessionReparent();
  if (sessionId === null) return null;
  return <OpenSessionParentPicker key={sessionId} reparent={reparent} sessionId={sessionId} />;
}

function OpenSessionParentPicker({
  sessionId,
  reparent,
}: {
  sessionId: string;
  reparent: SessionReparent;
}) {
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
  // profile that is not in Later; the catalog applies those scopes before the cut.
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
    },
    source !== null && sessionMoveUnderRefusal(source) === null,
    { facets: false, lens: source ? { profile: source.profile_name } : params }
  );
  const choose = (parent: SessionPayload) => {
    if (!source) return;
    reparent.adopt(source, parent);
    closeSessionParentPicker();
  };
  const makeTopLevel = () => {
    if (!source) return;
    reparent.detach(source);
    closeSessionParentPicker();
  };
  return (
    <SessionParentPicker
      candidates={source ? sessionParentCandidates(source, catalog.sessions) : []}
      failed={catalog.failed}
      loading={catalog.loading}
      // Rows of a previous search stay on screen while the next one loads; they
      // must not be chosen by an Enter meant for the new query.
      stale={catalog.paging || search !== remoteSearch}
      hasNext={search === remoteSearch && catalog.next}
      hasPrevious={search === remoteSearch && catalog.previous}
      onNextPage={catalog.nextPage}
      onPreviousPage={catalog.previousPage}
      onChoose={choose}
      onMakeTopLevel={makeTopLevel}
      onOpenChange={open => {
        if (!open) closeSessionParentPicker();
      }}
      onRetry={catalog.retry}
      onSearchChange={setSearch}
      open
      pending={reparent.pending}
      search={search}
      source={source}
      sourceFailed={sourceQuery.isError}
    />
  );
}
