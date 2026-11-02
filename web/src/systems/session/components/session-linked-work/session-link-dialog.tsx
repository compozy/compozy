/** One screen to link work by hand: paste a link or search, tick what belongs, done. */
import { Check, Link2, Unlink } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Checkbox,
  Command,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Empty,
  Icon,
  Pill,
  Spinner,
} from "@compozy/ui";

import { repositoryShortName, type SessionLinkSearch } from "../../lib/session-link-input";
import {
  candidateIsLinked,
  candidateKey,
  linkForkTitle,
  type SessionWorkCandidate,
} from "../../lib/session-linked-work";

const GROUP_CLASS =
  "p-0 **:[[cmdk-group-heading]]:px-3 **:[[cmdk-group-heading]]:pt-2 **:[[cmdk-group-heading]]:pb-2";

const CANDIDATE_STATE_LABEL: Record<string, string> = {
  open: "Open",
  draft: "Draft",
  merged: "Merged",
  closed: "Closed",
};

function candidateStateLabel(candidate: SessionWorkCandidate): string {
  if (!candidate.pr_number) return "No pull request yet";
  return CANDIDATE_STATE_LABEL[candidate.state ?? ""] ?? "Open";
}

function Candidate({
  candidate,
  checked,
  linked,
  onToggle,
}: {
  candidate: SessionWorkCandidate;
  checked: boolean;
  linked: boolean;
  onToggle: () => void;
}) {
  const number = candidate.pr_number;
  return (
    <CommandItem
      // The checkbox is decoration, and cmdk spends `aria-selected` on the highlighted row.
      aria-checked={linked ? undefined : checked}
      className="min-h-14 gap-3 rounded-lg px-3 py-2.5"
      data-slot="session-link-candidate"
      disabled={linked}
      onSelect={onToggle}
      value={candidateKey(candidate)}
    >
      {linked ? (
        <Icon as={Check} className="size-4 shrink-0 text-subtle" />
      ) : (
        <Checkbox
          aria-hidden="true"
          checked={checked}
          className="pointer-events-none"
          tabIndex={-1}
        />
      )}
      <span className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="truncate text-small-body font-medium text-fg">
          {candidate.title || candidate.branch}
        </span>
        <span className="flex min-w-0 items-center gap-1.5 text-eyebrow text-subtle">
          <Pill className="shrink-0" form="plain" tone="neutral">
            {candidateStateLabel(candidate)}
          </Pill>
          {candidate.title ? (
            <>
              <span aria-hidden="true" className="text-faint">
                ·
              </span>
              <span className="truncate">{candidate.branch}</span>
            </>
          ) : null}
        </span>
      </span>
      <span className="shrink-0 font-mono text-mono-id tracking-mono-id text-faint tabular-nums">
        {linked || !number ? null : `#${number}`}
      </span>
      {linked ? <span className="shrink-0 text-eyebrow text-subtle">Already linked</span> : null}
    </CommandItem>
  );
}

function NoResult({
  search,
  project,
  forge,
}: {
  search: Exclude<SessionLinkSearch, { kind: "results" }>;
  project: string;
  forge: string;
}) {
  return search.kind === "other-project" ? (
    <Empty
      className="py-8"
      data-slot="session-link-other-project"
      description={`This session is in ${project}. Open a session in that project to link it there.`}
      fill={false}
      icon={Unlink}
      size="compact"
      title={`#${search.number} belongs to ${repositoryShortName(search.repository)}`}
    />
  ) : (
    <Empty
      className="py-8"
      data-slot="session-link-unknown"
      description={`Check the number, or copy the full link from ${forge} and paste it here.`}
      fill={false}
      icon={Unlink}
      size="compact"
      title={`Can't find ${search.query}`}
    />
  );
}

export interface SessionLinkDialogProps {
  query: string;
  search: SessionLinkSearch;
  /** The first read for this query is still out. */
  loading: boolean;
  /** Pull request search is unavailable: only branches can be offered. */
  offline: boolean;
  /** The forge's name for copy, from the repository's host. */
  forge: string;
  /** Pull requests the last confirm was refused because they come from a fork. */
  forks: readonly number[];
  /** The session project's short name for copy. */
  project: string;
  linked: ReadonlySet<string>;
  selected: readonly SessionWorkCandidate[];
  hasMore: boolean;
  pending: boolean;
  /** Why the last confirm failed, when the search itself can't say it. */
  error: string | null;
  onQueryChange: (query: string) => void;
  onToggle: (candidate: SessionWorkCandidate) => void;
  onLoadMore: () => void;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
}

export function SessionLinkDialog({
  query,
  search,
  loading,
  offline,
  forge,
  forks,
  project,
  linked,
  selected,
  hasMore,
  pending,
  error,
  onQueryChange,
  onToggle,
  onLoadMore,
  onConfirm,
  onOpenChange,
}: SessionLinkDialogProps) {
  const chosen = new Set(selected.map(candidateKey));
  const row = (candidate: SessionWorkCandidate) => (
    <Candidate
      candidate={candidate}
      checked={chosen.has(candidateKey(candidate))}
      key={candidateKey(candidate)}
      linked={candidateIsLinked(candidate, linked)}
      onToggle={() => onToggle(candidate)}
    />
  );
  return (
    <Dialog onOpenChange={onOpenChange} open>
      <DialogContent className="sm:max-w-150" data-testid="session-link-dialog" unframed>
        <DialogHeader variant="ruled">
          <DialogTitle>Link a pull request or branch</DialogTitle>
          <DialogDescription>
            It shows up on this session. The agent keeps running where it is.
          </DialogDescription>
        </DialogHeader>
        <Command className="min-w-0 rounded-none bg-transparent" shouldFilter={false}>
          <div className="flex flex-col gap-3 px-5 pt-4 pb-3">
            {offline ? (
              <Alert variant="warning">
                <AlertTitle>{forge} isn't connected</AlertTitle>
                <AlertDescription>
                  You can still link a branch. Its pull request shows up once {forge} is back.
                </AlertDescription>
              </Alert>
            ) : null}
            {forks.length > 0 ? (
              <Alert variant="destructive">
                <AlertTitle>{linkForkTitle(forks)}</AlertTitle>
                <AlertDescription>Pull requests from forks can't be linked yet.</AlertDescription>
              </Alert>
            ) : null}
            {error ? (
              <Alert variant="destructive">
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            ) : null}
            <CommandInput
              autoFocus
              onValueChange={onQueryChange}
              placeholder="Paste a link, or search by number, title or branch"
              value={query}
            />
          </div>
          <CommandList className="h-96 max-h-[48vh] px-3 pb-3">
            {search.kind === "results" ? (
              <>
                {search.pullRequests.length > 0 ? (
                  <CommandGroup className={GROUP_CLASS} heading="Pull requests">
                    {search.pullRequests.map(row)}
                  </CommandGroup>
                ) : null}
                {search.branches.length > 0 ? (
                  <CommandGroup className={GROUP_CLASS} heading="Branches with no pull request">
                    {search.branches.map(row)}
                  </CommandGroup>
                ) : null}
                {loading ? (
                  <Spinner aria-label="Searching" className="mx-auto my-6 size-4" />
                ) : null}
                {hasMore ? (
                  <Button
                    className="mx-auto mt-2 flex"
                    onClick={onLoadMore}
                    size="xs"
                    type="button"
                    variant="quiet"
                  >
                    Show more
                  </Button>
                ) : null}
              </>
            ) : (
              <NoResult forge={forge} project={project} search={search} />
            )}
          </CommandList>
        </Command>
        <DialogFooter className="items-center sm:justify-between" variant="ruled">
          <span className="text-small-body text-muted-foreground">
            {selected.length > 0 ? `${selected.length} selected` : null}
          </span>
          <span className="flex items-center gap-2">
            <Button onClick={() => onOpenChange(false)} type="button" variant="secondary">
              Cancel
            </Button>
            <Button disabled={selected.length === 0 || pending} onClick={onConfirm} type="button">
              <Icon as={Link2} data-icon="inline-start" size="sm" />
              {selected.length > 1 ? `Link ${selected.length}` : "Link"}
            </Button>
          </span>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
