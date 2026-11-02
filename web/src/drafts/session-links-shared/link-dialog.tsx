/** One screen to link work by hand: paste a link or search, tick what belongs, done. */
import { Check, Link2, Unlink } from "@compozy/ui/icons";
import { useState } from "react";

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
  MonoId,
} from "@compozy/ui";

import { ChipBody } from "@/systems/session/components/session-pull-request-chip";
import type { SessionLink } from "@/systems/session/mocks/session-links-fixtures";

import { linkChip, linkTitle, matchesLink } from "./link-model";

const THIS_REPOSITORY = "compozy/compozy";
const LINK_URL = /github\.com\/([^/\s]+\/[^/\s]+)\/pull\/(\d+)/;

export interface LinkDialogPreset {
  query?: string;
  selected?: readonly string[];
  /** GitHub can't be reached: only branches can be offered. */
  offline?: boolean;
}

type LinkSearch =
  | { kind: "results"; matches: SessionLink[] }
  | { kind: "other-project"; repository: string; number: string }
  | { kind: "unknown"; query: string };

/** A pasted link wins over text search: it either names one pull request or it doesn't. */
function runSearch(candidates: readonly SessionLink[], query: string): LinkSearch {
  const pasted = LINK_URL.exec(query);
  if (pasted && pasted[1] !== THIS_REPOSITORY) {
    return { kind: "other-project", repository: pasted[1]!, number: pasted[2]! };
  }
  const needle = pasted ? pasted[2]! : query;
  const matches = candidates.filter(link => matchesLink(link, needle));
  return matches.length > 0 || query.trim() === ""
    ? { kind: "results", matches }
    : { kind: "unknown", query: query.trim() };
}

function Candidate({
  link,
  checked,
  linked,
  onToggle,
}: {
  link: SessionLink;
  checked: boolean;
  linked: boolean;
  onToggle: () => void;
}) {
  const chip = linkChip(link);
  return (
    <CommandItem
      className="min-h-11 gap-2.5 py-1.5"
      disabled={linked}
      onSelect={onToggle}
      value={link.id}
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
        <span className="truncate text-small-body text-fg">{linkTitle(link)}</span>
        {link.title ? (
          <span className="flex min-w-0 items-center gap-1.5 overflow-hidden whitespace-nowrap text-badge font-medium text-muted-foreground [&_svg]:size-3 [&_svg]:shrink-0">
            <ChipBody model={chip} refreshing={false} />
          </span>
        ) : null}
      </span>
      {linked ? (
        <span className="shrink-0 text-eyebrow text-subtle">Already linked</span>
      ) : link.title ? (
        <MonoId
          className="max-w-44 shrink truncate"
          preserveCase
          size="sm"
          value={link.facts.branch ?? ""}
        />
      ) : null}
    </CommandItem>
  );
}

function NoResult({ search }: { search: Exclude<LinkSearch, { kind: "results" }> }) {
  return search.kind === "other-project" ? (
    <Empty
      className="py-8"
      description={`This session is in ${THIS_REPOSITORY}. Open a session in that project to link it there.`}
      fill={false}
      icon={Unlink}
      size="compact"
      title={`#${search.number} belongs to ${search.repository}`}
    />
  ) : (
    <Empty
      className="py-8"
      description="Check the number, or copy the full link from GitHub and paste it here."
      fill={false}
      icon={Unlink}
      size="compact"
      title={`Can't find ${search.query}`}
    />
  );
}

export interface LinkDialogProps {
  candidates: readonly SessionLink[];
  linked: readonly SessionLink[];
  preset?: LinkDialogPreset;
  onLink: (added: readonly SessionLink[]) => void;
  onOpenChange: (open: boolean) => void;
}

export function LinkDialog({ candidates, linked, preset, onLink, onOpenChange }: LinkDialogProps) {
  const [query, setQuery] = useState(preset?.query ?? "");
  const [selected, setSelected] = useState<readonly string[]>(preset?.selected ?? []);
  const offered = preset?.offline ? candidates.filter(link => !link.title) : candidates;
  const search = runSearch(offered, query);
  const linkedIds = new Set(linked.map(link => link.id));
  const toggle = (id: string) =>
    setSelected(current =>
      current.includes(id) ? current.filter(item => item !== id) : [...current, id]
    );
  const matches = search.kind === "results" ? search.matches : [];
  const pullRequests = matches.filter(link => link.title);
  const branches = matches.filter(link => !link.title);
  const row = (link: SessionLink) => (
    <Candidate
      checked={selected.includes(link.id)}
      key={link.id}
      link={link}
      linked={linkedIds.has(link.id)}
      onToggle={() => toggle(link.id)}
    />
  );
  return (
    <Dialog onOpenChange={onOpenChange} open>
      <DialogContent className="sm:max-w-140">
        <DialogHeader>
          <DialogTitle>Link a pull request or branch</DialogTitle>
          <DialogDescription>
            It shows up on this session. The agent keeps running where it is.
          </DialogDescription>
        </DialogHeader>
        {preset?.offline ? (
          <Alert variant="warning">
            <AlertTitle>GitHub isn't connected</AlertTitle>
            <AlertDescription>
              You can still link a branch. Its pull request shows up once GitHub is back.
            </AlertDescription>
          </Alert>
        ) : null}
        <Command className="min-w-0 bg-transparent" shouldFilter={false}>
          <CommandInput
            autoFocus
            onValueChange={setQuery}
            placeholder="Paste a link, or search by number, title or branch"
            value={query}
          />
          <CommandList className="h-80 max-h-[46vh] pt-2">
            {search.kind === "results" ? (
              <>
                {pullRequests.length > 0 ? (
                  <CommandGroup heading="Pull requests">{pullRequests.map(row)}</CommandGroup>
                ) : null}
                {branches.length > 0 ? (
                  <CommandGroup heading="Branches with no pull request">
                    {branches.map(row)}
                  </CommandGroup>
                ) : null}
              </>
            ) : (
              <NoResult search={search} />
            )}
          </CommandList>
        </Command>
        <DialogFooter>
          <Button onClick={() => onOpenChange(false)} type="button" variant="secondary">
            Cancel
          </Button>
          <Button
            disabled={selected.length === 0}
            onClick={() => {
              const chosen = new Set(selected);
              onLink(candidates.filter(link => chosen.has(link.id)));
              onOpenChange(false);
            }}
            type="button"
          >
            <Icon as={Link2} data-icon="inline-start" size="sm" />
            {selected.length > 1 ? `Link ${selected.length}` : "Link"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
