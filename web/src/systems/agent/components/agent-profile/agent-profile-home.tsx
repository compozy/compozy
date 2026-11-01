/** S4: the agent's home. Who it is, what it's doing, and what it has done for you. */
import { ArrowRight, MessageSquarePlus, MessagesSquare, Play } from "@compozy/ui/icons";

import { Alert, AlertDescription, AlertTitle, Button, type AgentAvatarState } from "@compozy/ui";

import type { UseAgentProfile } from "../../hooks/use-agent-profile";
import { agentCanResume } from "../../lib/agent-profile";
import { AGENT_PAUSE_PLAIN, type AgentTeamPlace } from "../../lib/agent-team";
import { AgentDiagnosticsBanner } from "../agent-diagnostics-banner";
import {
  AgentActivitySection,
  AgentActivityUnavailable,
  AgentFirstDay,
  AgentPlanCard,
  AgentRhythmSection,
} from "./agent-profile-activity";
import { AgentProfileHero } from "./agent-profile-hero";
import { AgentProfilePanels } from "./agent-profile-panels";
import { AgentProfileSetup } from "./agent-profile-setup";
import { AgentFindingsPanel, AgentRecentWork } from "./agent-profile-work";
import { AgentAddPlace, AgentWorkingNow } from "./agent-where-it-works";

function MainAction({ page, state }: { page: UseAgentProfile; state: AgentAvatarState }) {
  const { identity, places } = page;
  if (!identity.staysOn) {
    return (
      <Button
        data-testid="agent-page-new-session"
        disabled={page.newSessionDisabled}
        onClick={page.onNewSession}
        size="sm"
        type="button"
        variant="secondary"
      >
        <MessageSquarePlus aria-hidden="true" />
        New session
      </Button>
    );
  }
  if (places.length === 0) {
    return (
      <AgentAddPlace
        add={page.add}
        name={identity.name}
        places={places}
        primary
        provider={identity.provider}
      />
    );
  }
  const needing = places.find(place => place.state === "needs-you");
  if (needing) {
    return (
      <Button onClick={() => page.placeActions.onOpen(needing)} size="sm" type="button">
        See what it needs
        <ArrowRight aria-hidden="true" />
      </Button>
    );
  }
  if (state === "paused") {
    if (!places.some(place => agentCanResume(place.pauseReason))) return null;
    return (
      <Button onClick={page.onResumeAll} size="sm" type="button" variant="secondary">
        <Play aria-hidden="true" />
        Resume
      </Button>
    );
  }
  return (
    <Button
      onClick={() => page.placeActions.onOpen(places[0]!)}
      size="sm"
      type="button"
      variant="secondary"
    >
      <MessagesSquare aria-hidden="true" />
      Open session
    </Button>
  );
}

function Stopped({ places }: { places: readonly AgentTeamPlace[] }) {
  const stopped = places.find(place => place.pauseReason && place.pauseReason !== "manual");
  if (!stopped?.pauseReason) return null;
  return (
    <Alert data-testid="agent-profile-stopped" variant="warning">
      <AlertTitle>
        {agentCanResume(stopped.pauseReason) ? "Check-ins kept failing" : "It can't resume"}
      </AlertTitle>
      <AlertDescription>{AGENT_PAUSE_PLAIN[stopped.pauseReason]}</AlertDescription>
    </Alert>
  );
}

function Going({ page }: { page: UseAgentProfile }) {
  const { activity, identity, fresh } = page;
  if (activity === undefined) return null;
  if (activity === null) return <AgentActivityUnavailable />;
  if (fresh) return <AgentFirstDay name={identity.label} staysOn={identity.staysOn} />;
  return (
    <>
      <AgentActivitySection
        activity={activity}
        label={identity.label}
        onDemand={!identity.staysOn}
      />
      <AgentRhythmSection activity={activity} />
    </>
  );
}

export function AgentProfileHome({ page }: { page: UseAgentProfile }) {
  const { identity, places, state } = page;
  return (
    <div className="@container flex flex-col gap-6" data-testid="agent-detail-page">
      <AgentProfileHero
        action={<MainAction page={page} state={state} />}
        identity={identity}
        line={page.headline}
        onSave={page.onSaveIdentity}
        state={state}
      />
      {page.diagnostics.length > 0 ? (
        <AgentDiagnosticsBanner diagnostics={page.diagnostics} />
      ) : null}
      <Stopped places={places} />
      <div className="grid grid-cols-1 items-start gap-5 @5xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex min-w-0 flex-col gap-6">
          {identity.staysOn && places.length > 0 ? (
            <AgentWorkingNow
              actions={page.placeActions}
              add={page.add}
              name={identity.name}
              onSeeAll={page.onOpenWhere}
              places={places}
              provider={identity.provider}
            />
          ) : null}
          <Going page={page} />
        </div>
        <aside className="flex min-w-0 flex-col gap-6">
          <AgentFindingsPanel findings={page.findings} onOpen={page.onOpenFinding} />
          <AgentRecentWork items={page.work} onOpen={page.onOpenSession} />
          {page.fresh ? null : <AgentPlanCard provider={identity.provider} />}
          <AgentProfileSetup
            onOpenBehavior={() => page.onOpenEditor("what-it-does")}
            onOpenCheckIns={() => page.onOpenEditor("check-ins")}
            onOpenMemory={() => page.onOpenEditor("memory")}
            onOpenModel={() => page.setPanel("model")}
            onOpenTools={() => page.setPanel("tools")}
            owner={identity.owner}
            readOnly={identity.readOnly}
            staysOn={identity.staysOn}
            summaries={page.summaries}
          />
        </aside>
      </div>
      <AgentProfilePanels
        label={identity.label}
        name={identity.name}
        onClose={() => page.setPanel(null)}
        owner={identity.owner}
        panel={page.panel}
        provider={identity.provider}
        readOnly={identity.readOnly}
      />
    </div>
  );
}
