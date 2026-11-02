import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { useDebouncedValue } from "@/hooks/use-debounced-input";

import { isForkUnsupported, isRepositoryMismatch } from "../adapters/session-linked-work-api";
import type { SessionLinkDialogProps } from "../components/session-linked-work/session-link-dialog";
import {
  hostedRepository,
  parseSessionLinkInput,
  repositoryForgeName,
  repositoryShortName,
  sessionLinkExactMatch,
  sessionLinkSearch,
  sessionLinkSearchTerm,
} from "../lib/session-link-input";
import {
  candidateKey,
  linkedCandidateKeys,
  linkFailureMessage,
  linkRequest,
  type SessionLinkFailure,
  type SessionWorkCandidate,
} from "../lib/session-linked-work";
import {
  sessionLinkCandidatesOptions,
  sessionLinkedWorkOptions,
} from "../lib/session-linked-work-options";
import { settleBounded } from "../lib/settle-bounded";
import { closeSessionLinkDialog } from "./use-session-link-dialog";
import { useSessionLinkMutations } from "./use-session-linked-work";

/** Order doesn't matter to the daemon, so a selection links in parallel, gently. */
const LINK_CONCURRENCY = 3;

/** The picker's two server searches for the settled query. */
function useSessionLinkCandidates(workspaceId: string, sessionId: string, query: string) {
  const term = sessionLinkSearchTerm(query);
  const pulls = useInfiniteQuery(
    sessionLinkCandidatesOptions(workspaceId, sessionId, "pull_requests", term)
  );
  const branchReads = useInfiniteQuery(
    sessionLinkCandidatesOptions(workspaceId, sessionId, "branches", term)
  );
  const pages = pulls.data?.pages ?? [];
  const discovery = pages[0]?.supports_pr_discovery ?? true;
  // The refusal answers both searches: it is about the link, not about the forge.
  const refused = isRepositoryMismatch(pulls.error) || isRepositoryMismatch(branchReads.error);
  return {
    pullRequests: discovery ? pages.flatMap(page => page.items) : [],
    branches: branchReads.data?.pages.flatMap(page => page.items) ?? [],
    // A forge that can't answer leaves branches linkable; it is not a dead dialog.
    offline: (pulls.isError && !refused) || !discovery,
    refused,
    fetching: pulls.isFetching || branchReads.isFetching || pulls.isPlaceholderData,
    fetchingMore: pulls.isFetchingNextPage || branchReads.isFetchingNextPage,
    // The last search's rows, kept on screen until this one answers.
    outdated: pulls.isPlaceholderData || branchReads.isPlaceholderData,
    hasMore: pulls.hasNextPage || branchReads.hasNextPage,
    loadMore: () => {
      if (pulls.hasNextPage) void pulls.fetchNextPage();
      if (branchReads.hasNextPage) void branchReads.fetchNextPage();
    },
  };
}

/** What was picked, plus the pasted match that ticks itself until it is unticked. */
function useSessionLinkSelection(exact: SessionWorkCandidate | null) {
  const [picked, setPicked] = useState<readonly SessionWorkCandidate[]>([]);
  const [declined, setDeclined] = useState<string | null>(null);
  const exactKey = exact ? candidateKey(exact) : null;
  const pickedExact = picked.some(candidate => candidateKey(candidate) === exactKey);
  const selected = exact && exactKey !== declined && !pickedExact ? [...picked, exact] : picked;
  return {
    selected,
    /** Narrows the selection to what is left; a pasted match that went through stays unticked. */
    keep: (remaining: readonly SessionWorkCandidate[]) => {
      setPicked(remaining);
      if (!remaining.some(item => candidateKey(item) === exactKey)) setDeclined(exactKey);
    },
    toggle: (candidate: SessionWorkCandidate) => {
      const key = candidateKey(candidate);
      if (key === exactKey) setDeclined(current => (current === key ? null : key));
      setPicked(current =>
        current.some(item => candidateKey(item) === key) || key === exactKey
          ? current.filter(item => candidateKey(item) !== key)
          : [...current, candidate]
      );
    },
  };
}

interface UseSessionLinkDialogModelInput {
  sessionId: string;
  workspaceId: string;
  workspaceName: string;
  profile: string;
}

export function useSessionLinkDialogModel({
  sessionId,
  workspaceId,
  workspaceName,
  profile,
}: UseSessionLinkDialogModelInput): SessionLinkDialogProps {
  const [query, setQuery] = useState("");
  const [refused, setRefused] = useState(false);
  const [forks, setForks] = useState<readonly number[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);
  const debounced = useDebouncedValue(query.trim());
  const list = useQuery(sessionLinkedWorkOptions(workspaceId, sessionId));
  const candidates = useSessionLinkCandidates(workspaceId, sessionId, debounced);
  const { add } = useSessionLinkMutations(workspaceId, sessionId, profile);

  const { pullRequests, branches } = candidates;
  const items = list.data?.items ?? [];
  const repository =
    items[0]?.repository ?? pullRequests[0]?.repository ?? branches[0]?.repository ?? null;
  const hosted = hostedRepository(repository);
  const searching = query.trim() !== debounced || candidates.fetching;
  const exact = searching ? null : sessionLinkExactMatch(debounced, pullRequests);
  const selection = useSessionLinkSelection(exact);

  const confirm = async () => {
    setPending(true);
    setError(null);
    const { selected } = selection;
    try {
      const outcomes = await settleBounded(selected, LINK_CONCURRENCY, candidate =>
        add.mutateAsync(linkRequest(candidate))
      );
      const failures = selected.flatMap((candidate, index): SessionLinkFailure[] => {
        const outcome = outcomes[index];
        return outcome?.status === "rejected" ? [{ candidate, cause: outcome.reason }] : [];
      });
      // A fork is refused however often it is retried, so it leaves the selection.
      const retryable = failures.filter(({ cause }) => !isForkUnsupported(cause));
      setForks(
        failures.flatMap(({ candidate, cause }) =>
          isForkUnsupported(cause) && candidate.pr_number ? [candidate.pr_number] : []
        )
      );
      // A pasted link has its own "another project" state; anything else must say so in words.
      const pastedLink = parseSessionLinkInput(query).kind === "link";
      setRefused(failures.some(({ cause }) => isRepositoryMismatch(cause)));
      setError(
        linkFailureMessage(
          pastedLink ? retryable.filter(({ cause }) => !isRepositoryMismatch(cause)) : retryable
        )
      );
      selection.keep(retryable.map(({ candidate }) => candidate));
      if (failures.length === 0) closeSessionLinkDialog();
    } finally {
      setPending(false);
    }
  };

  return {
    error,
    forge: repositoryForgeName(hosted),
    forgeless: repository !== null && hosted === null,
    forks,
    fetchingMore: candidates.fetchingMore,
    hasMore: candidates.hasMore,
    linked: linkedCandidateKeys(items),
    outdated: candidates.outdated || query.trim() !== debounced,
    loading: searching && pullRequests.length + branches.length === 0,
    offline: candidates.offline,
    onConfirm: () => void confirm(),
    onLoadMore: candidates.loadMore,
    onOpenChange: open => {
      if (!open) closeSessionLinkDialog();
    },
    onQueryChange: value => {
      setQuery(value);
      setRefused(false);
      setForks([]);
      setError(null);
    },
    onToggle: selection.toggle,
    pending,
    project: hosted
      ? repositoryShortName(hosted)
      : workspaceName.split("/").at(-1) || workspaceName,
    query,
    search: sessionLinkSearch({
      query: debounced,
      repository,
      pullRequests,
      branches,
      searching,
      refused: refused || candidates.refused,
    }),
    selected: selection.selected,
  };
}
