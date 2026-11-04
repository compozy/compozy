/**
 * The Agents window and Merge Train's home inside the app shell, with the bell opened from the real
 * system-bar trigger in the menubar's own popover.
 */
import { MessagesSquare, Moon } from "@compozy/ui/icons";
import type { ReactElement } from "react";

import {
  Button,
  Empty,
  Panel,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Section,
} from "@compozy/ui";

import { usePlaces } from "../agent-profile/hooks/use-places";
import { Hero } from "../agent-profile/profile-hero";
import { FindingsPanel, RecentWork } from "../agent-profile/profile-work";
import { WorkingNow } from "../agent-profile/where-it-works";
import { AgentAppPage } from "../always-on-agents-shared/agent-page";
import { AgentsPage } from "../always-on-agents-shared/agents-page";
import { ProfileAlert, ScriptWaitingPill } from "./agent-badges";
import { ScriptApprovalBellPanel, type BellVariant } from "./bell-variations";
import {
  FLEET_ON_DEMAND,
  MERGE_TRAIN,
  MERGE_TRAIN_PROFILE,
  SCRIPT_BELL_TOTAL,
  fleetAlwaysOn,
} from "./fleet-fixtures";

/** The bell trigger in `DesktopMenubar`'s popover, opened for review. */
function openBell(variant: BellVariant) {
  return function BellPopover(trigger: ReactElement) {
    return (
      <Popover defaultOpen>
        <PopoverTrigger render={trigger} />
        <PopoverContent
          align="end"
          aria-label="Attention"
          className="w-80 max-w-[calc(100vw-16px)] p-2"
        >
          <ScriptApprovalBellPanel variant={variant} />
        </PopoverContent>
      </Popover>
    );
  };
}

/** Agents › cards, Always-on group; `badges` adds the waiting pill to each agent it concerns. */
export function AgentsInShell({ bell, badges }: { bell?: BellVariant; badges: boolean }) {
  const pill = badges ? <ScriptWaitingPill /> : undefined;
  return (
    <AgentsPage
      alwaysOn={fleetAlwaysOn({ mergeTrain: pill, sessionSupervisor: pill })}
      initialView="cards"
      onDemand={FLEET_ON_DEMAND}
      shell={{
        notifications: SCRIPT_BELL_TOTAL,
        wrapBellTrigger: bell ? openBell(bell) : undefined,
      }}
    />
  );
}

/**
 * Merge Train's home with the alert that sends you to Heartbeat, from the `agent-profile` draft's
 * parts. Its Setup panel is left out: that draft reaches the daemon through production imports.
 */
export function ProfileInShell() {
  const data = MERGE_TRAIN_PROFILE;
  const { places, actions } = usePlaces(data.places);
  return (
    <AgentAppPage
      agent={MERGE_TRAIN}
      shell={{ notifications: SCRIPT_BELL_TOTAL }}
      width="max-w-6xl"
    >
      <div className="@container flex flex-col gap-6">
        <Hero
          action={
            <Button size="sm" type="button" variant="secondary">
              <MessagesSquare aria-hidden="true" />
              Open session
            </Button>
          }
          data={data}
          line="Resting in 1 project"
          state="idle"
        />
        <ProfileAlert />
        <div className="grid grid-cols-1 items-start gap-5 @5xl:grid-cols-[minmax(0,1fr)_22rem]">
          <div className="flex min-w-0 flex-col gap-6">
            <WorkingNow actions={actions} places={places} />
            <Section label="How it's been going">
              <Panel bodyClassName="px-6 py-14">
                <Empty
                  description="Everything it does, finds and costs will show up here."
                  fill={false}
                  icon={Moon}
                  title="Merge Train hasn't checked in yet"
                />
              </Panel>
            </Section>
          </div>
          <aside className="flex min-w-0 flex-col gap-6">
            <FindingsPanel data={data} />
            <RecentWork data={data} />
          </aside>
        </div>
      </div>
    </AgentAppPage>
  );
}
