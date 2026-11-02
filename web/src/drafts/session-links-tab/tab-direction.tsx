/**
 * Direction C. Every session gains a second page next to the conversation, with room for one
 * clear row per item at any scale. The message box stays, so the agent is one sentence away.
 */
import { useState } from "react";

import { LaneTabs } from "@compozy/ui";

import { SessionPullRequestChip } from "@/systems/session/components/session-pull-request-chip";

import { leadLink } from "../session-links-shared/link-model";
import { LinkStage } from "../session-links-shared/link-stage";
import { LinkedWork } from "../session-links-shared/linked-work";
import { linkScenario } from "../session-links-shared/scenarios";
import { useLinks } from "../session-links-shared/use-links";

type SessionPage = "conversation" | "linked";

export function TabDirection({ state }: { state: string }) {
  const scenario = linkScenario(state);
  const { links, unlink, add } = useLinks(scenario.links);
  const [page, setPage] = useState<SessionPage>(scenario.open ? "linked" : "conversation");
  const [dialogOpen, setDialogOpen] = useState(false);
  const lead = leadLink(links);
  return (
    <LinkStage
      dialogOpen={dialogOpen}
      links={links}
      onAdd={add}
      onDialogOpenChange={setDialogOpen}
      pane={
        page === "linked" ? (
          <div className="min-h-0 flex-1 overflow-y-auto">
            <div className="mx-auto w-full max-w-5xl px-5 py-5">
              <LinkedWork
                layout={scenario.narrow ? "tight" : "wide"}
                links={links}
                onLink={() => setDialogOpen(true)}
                onUnlink={unlink}
                openMenuFor={scenario.openMenuFor}
                runsIn={scenario.runsIn}
              />
            </div>
          </div>
        ) : undefined
      }
      scenario={scenario}
      status={lead ? <SessionPullRequestChip controllable facts={lead.facts} /> : null}
      strip={
        <LaneTabs
          ariaLabel="Session page"
          className="shrink-0 border-b border-line px-4"
          items={[
            { value: "conversation", label: "Conversation" },
            { value: "linked", label: "Linked work", count: links.length },
          ]}
          onChange={setPage}
          value={page}
        />
      }
    />
  );
}
