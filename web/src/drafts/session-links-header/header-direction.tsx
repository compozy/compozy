/**
 * The header keeps today's chip, naming the most urgent item, and `+3 PRs` beside it opens the
 * list. Nothing new at rest, nothing stays open.
 */
import { FolderGit2, GitPullRequest, Link2, Unlink } from "@compozy/ui/icons";
import { useState, type ReactElement, type ReactNode } from "react";

import {
  Button,
  DropdownMenuItem,
  DropdownMenuLabel,
  Icon,
  Popover,
  PopoverContent,
  PopoverTrigger,
  cn,
} from "@compozy/ui";

import { SessionPullRequestChip } from "@/systems/session/components/session-pull-request-chip";
import { SESSION_CHIP_CLASS } from "@/systems/session/components/session-pull-request-chip-style";

import { leadLink, moreLabel } from "../session-links-shared/link-model";
import { LinkStage } from "../session-links-shared/link-stage";
import { LinkedWork } from "../session-links-shared/linked-work";
import { linkScenario } from "../session-links-shared/scenarios";
import { useLinks } from "../session-links-shared/use-links";

export function HeaderDirection({ state }: { state: string }) {
  const scenario = linkScenario(state);
  const { links, unlink, add } = useLinks(scenario.links);
  const [dialogOpen, setDialogOpen] = useState(false);
  const lead = leadLink(links);
  const more = moreLabel(links);
  const openLink = () => setDialogOpen(true);
  const list = (trigger: ReactElement, children: ReactNode) => (
    <Popover defaultOpen={scenario.open}>
      <PopoverTrigger data-testid="session-links-more" render={trigger}>
        {children}
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-120 max-w-[calc(100vw-2rem)] gap-0 overflow-hidden p-0"
      >
        <LinkedWork
          links={links}
          onLink={openLink}
          onUnlink={unlink}
          openMenuFor={scenario.openMenuFor}
          runsIn={scenario.runsIn}
        />
      </PopoverContent>
    </Popover>
  );
  // The head clips its status zone first, so a narrow window keeps the list on the actions side.
  const headAction =
    scenario.narrow && links.length > 0
      ? list(
          <Button
            aria-label={`${links.length} linked to this session`}
            size="sm"
            type="button"
            variant="quiet"
          />,
          <>
            <Icon as={GitPullRequest} data-icon="inline-start" size="sm" />
            {links.length}
          </>
        )
      : undefined;
  return (
    <LinkStage
      dialogOpen={dialogOpen}
      environmentOpen={state === "none"}
      headAction={headAction}
      links={links}
      onAdd={add}
      onDialogOpenChange={setDialogOpen}
      scenario={scenario}
      status={
        lead && !scenario.narrow ? (
          <span className="flex min-w-0 items-center gap-1">
            <SessionPullRequestChip
              controllable
              defaultMenuOpen={links.length === 1 && scenario.open}
              facts={lead.facts}
              menuFooter={
                more ? undefined : (
                  <>
                    <DropdownMenuLabel className="tracking-normal normal-case">
                      This session runs in {scenario.runsIn.name}
                    </DropdownMenuLabel>
                    {lead.worktree ? (
                      <DropdownMenuItem>
                        <Icon as={FolderGit2} size="sm" />
                        Worktree {lead.worktree.name}
                      </DropdownMenuItem>
                    ) : null}
                    <DropdownMenuItem onClick={openLink}>
                      <Icon as={Link2} size="sm" />
                      Link another…
                    </DropdownMenuItem>
                    <DropdownMenuItem onClick={() => unlink(lead)}>
                      <Icon as={Unlink} size="sm" />
                      Unlink from this session
                    </DropdownMenuItem>
                  </>
                )
              }
            />
            {more
              ? list(
                  <Button
                    className={cn(SESSION_CHIP_CLASS, "pr-2.5")}
                    size="sm"
                    type="button"
                    variant="secondary"
                  />,
                  more
                )
              : null}
          </span>
        ) : null
      }
    />
  );
}
