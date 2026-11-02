/** The real palette rows: a PR number finds its session, and linking is one command away. */
import { Link2 } from "@compozy/ui/icons";
import { useState } from "react";

import {
  Command,
  CommandDialog,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Icon,
} from "@compozy/ui";

import { OsPaletteEntitySections } from "@/systems/os/components/os-palette-entity-sections";
import { OsPaletteFooter } from "@/systems/os/components/os-palette-footer";
import {
  paletteGroupClass,
  paletteHeadClass,
  paletteInputRailClass,
  paletteListClass,
  paletteRowClass,
} from "@/systems/os/lib/palette-view-inset";
import { SessionPullRequestChip } from "@/systems/session/components/session-pull-request-chip";

import { leadLink } from "../session-links-shared/link-model";
import { LinkStage } from "../session-links-shared/link-stage";
import { linkScenario } from "../session-links-shared/scenarios";
import { useLinks } from "../session-links-shared/use-links";

const FOUND = {
  sessions: [
    {
      sessionId: "s4",
      title: "Topbar PR chip — web slice",
      agentName: "claude",
      workspaceLabel: "#944 · compozy",
      busy: false,
      workspaceId: "ws-compozy",
      route: { pathname: "/sessions/s4", search: {} },
    },
  ],
  sessionTotal: 1,
  tabs: [],
  tabTotal: 0,
  worktrees: [],
  worktreeTotal: 0,
  selectWorktree: () => undefined,
};

export function PaletteSurface({ mode }: { mode: "find" | "link" }) {
  const scenario = linkScenario("mixed-rest");
  const { links, add } = useLinks(scenario.links);
  const [dialogOpen, setDialogOpen] = useState(false);
  const [open, setOpen] = useState(true);
  const lead = leadLink(links);
  return (
    <LinkStage
      dialogOpen={dialogOpen}
      links={links}
      onAdd={add}
      onDialogOpenChange={setDialogOpen}
      overlay={
        <CommandDialog
          className="sm:max-w-160"
          onOpenChange={setOpen}
          open={open}
          title="Command palette"
        >
          <Command className={paletteInputRailClass} shouldFilter={false}>
            <div className={paletteHeadClass}>
              <CommandInput
                placeholder="Search apps, sessions, and actions…"
                value={mode === "find" ? "#944" : "link"}
                variant="quiet"
              />
            </div>
            <CommandList
              className={`${paletteListClass} **:data-[testid=os-palette-section-sessions]:mt-0 **:data-[testid=os-palette-section-sessions]:border-t-0 **:data-[testid=os-palette-section-sessions]:pt-0`}
            >
              {mode === "find" ? (
                <OsPaletteEntitySections
                  destination={false}
                  entities={FOUND}
                  onGoToTab={() => undefined}
                  onOpenSession={() => undefined}
                  onSelectWorktree={() => undefined}
                />
              ) : (
                <CommandGroup className={paletteGroupClass} heading="This session">
                  <CommandItem
                    className={paletteRowClass}
                    onSelect={() => {
                      setOpen(false);
                      setDialogOpen(true);
                    }}
                    value="link"
                  >
                    <Icon as={Link2} className="size-3.5 text-muted-foreground" />
                    Link a pull request or branch…
                  </CommandItem>
                </CommandGroup>
              )}
            </CommandList>
            <OsPaletteFooter enterHint="open" />
          </Command>
        </CommandDialog>
      }
      scenario={scenario}
      status={lead ? <SessionPullRequestChip controllable facts={lead.facts} /> : null}
    />
  );
}
