/**
 * Direction B. The list lives in the session's existing side panel, closed by default, so it can
 * stay beside the conversation. Several items reduce the header to one summary that opens it.
 */
import { GitPullRequest } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, Pill, cn } from "@compozy/ui";

import { SessionPullRequestChip } from "@/systems/session/components/session-pull-request-chip";
import { SESSION_CHIP_CLASS } from "@/systems/session/components/session-pull-request-chip-style";
import { DetailInspector } from "@compozy/ui";
import { SessionContextInjectedSection } from "@/systems/session/components/session-context-injected-section";
import { SessionContextMeterBody } from "@/systems/session/components/session-context-meter-body";
import { SessionInspectorSection } from "@/systems/session/components/session-inspector-section";
import { deriveSessionContext } from "@/systems/session/lib/session-context";
import { describeSessionContextMeter } from "@/systems/session/lib/session-context-view";
import { sessionContextFixture } from "@/systems/session/mocks/context-fixtures";

import { leadLink, needsYouCount, needsYouLabel } from "../session-links-shared/link-model";
import { LinkStage } from "../session-links-shared/link-stage";
import { LinkedWork } from "../session-links-shared/linked-work";
import { linkScenario } from "../session-links-shared/scenarios";
import { useLinks } from "../session-links-shared/use-links";

const context = deriveSessionContext(sessionContextFixture);

function SummaryChip({
  count,
  needsYou,
  onClick,
}: {
  count: number;
  needsYou: number;
  onClick: () => void;
}) {
  return (
    <Button
      className={cn(SESSION_CHIP_CLASS, "pr-2.5")}
      data-testid="session-links-summary"
      onClick={onClick}
      size="sm"
      type="button"
      variant="secondary"
    >
      <GitPullRequest aria-hidden="true" className="text-subtle" />
      {count} linked
      {needsYou > 0 ? (
        <>
          <span aria-hidden="true" className="text-faint">
            ·
          </span>
          <span className="inline-flex items-center gap-1 text-fg-2">
            <Pill.Dot size="sm" tone="brand" />
            {needsYouLabel(needsYou)}
          </span>
        </>
      ) : null}
    </Button>
  );
}

export function PanelDirection({ state }: { state: string }) {
  const scenario = linkScenario(state);
  const { links, unlink, add } = useLinks(scenario.links);
  const [panelOpen, setPanelOpen] = useState(scenario.open);
  const [dialogOpen, setDialogOpen] = useState(false);
  const lead = leadLink(links);
  return (
    <LinkStage
      dialogOpen={dialogOpen}
      inspector={{ open: panelOpen, onToggle: () => setPanelOpen(!panelOpen) }}
      links={links}
      onAdd={add}
      onDialogOpenChange={setDialogOpen}
      scenario={scenario}
      side={
        panelOpen ? (
          <DetailInspector
            className="min-w-0"
            drawerClassName="rounded-none border-l border-line-strong"
            onOpenChange={setPanelOpen}
            open={panelOpen}
            title="Context"
          >
            <div className="flex flex-col gap-4.5 p-4">
              <LinkedWork
                layout="tight"
                links={links}
                onLink={() => setDialogOpen(true)}
                onUnlink={unlink}
                openMenuFor={scenario.openMenuFor}
                runsIn={scenario.runsIn}
              />
              <SessionInspectorSection>
                <SessionContextMeterBody view={describeSessionContextMeter(context)} />
              </SessionInspectorSection>
              <SessionContextInjectedSection injected={context.injected} showBars />
            </div>
          </DetailInspector>
        ) : null
      }
      status={
        links.length > 1 ? (
          <SummaryChip
            count={links.length}
            needsYou={needsYouCount(links)}
            onClick={() => setPanelOpen(!panelOpen)}
          />
        ) : lead ? (
          <SessionPullRequestChip controllable facts={lead.facts} />
        ) : null
      }
    />
  );
}
