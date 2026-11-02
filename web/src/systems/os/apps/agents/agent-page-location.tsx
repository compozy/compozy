import { PageContent } from "@compozy/ui";

import {
  AgentAddPlace,
  AgentCheckInsEditor,
  AgentMemoryEditor,
  AgentStartingInstructions,
  AgentWhatItDoesEditor,
  AgentWhereItWorks,
  isAgentReadOnly,
  useAgentProfile,
  useAgentWindowScope,
  type AgentEditorPage,
  type AgentPayload,
} from "@/systems/agent";

import { AgentDetailLoading, AgentNotFound } from "./agent-detail-location";
import { AGENT_PAGE_SCROLL_CLASS } from "./agent-page-frame";
import { useAgentDetail, type UseAgentDetailResult } from "./use-agent-detail";
import { useAgentDetailTopbar } from "./use-agent-detail-topbar";

const PAGE_TITLES: Record<AgentEditorPage, string> = {
  "what-it-does": "What it does",
  "check-ins": "Check-ins",
  memory: "Memory",
  where: "Where it works",
};

function noPanel() {
  return undefined;
}

function PageBody({
  agent,
  page,
  detail,
}: {
  agent: AgentPayload;
  page: AgentEditorPage;
  detail: UseAgentDetailResult;
}) {
  const { workspace } = useAgentWindowScope();
  const profile = useAgentProfile({
    agent,
    panel: null,
    setPanel: noPanel,
    active: detail.active,
    memorySummary: "",
  });
  const props = {
    agent,
    onAddToProject: profile.onOpenWhere,
    readOnly: isAgentReadOnly(agent),
    state: profile.state,
    workspace,
  };
  useAgentDetailTopbar(detail, agent.name, PAGE_TITLES[page], {
    actions:
      page === "where" ? (
        <AgentAddPlace
          add={profile.add}
          name={agent.name}
          places={profile.places}
          primary
          provider={agent.provider || null}
        />
      ) : undefined,
  });
  switch (page) {
    case "what-it-does":
      return (
        <>
          <AgentWhatItDoesEditor {...props} />
          <AgentStartingInstructions
            agent={agent}
            readOnly={props.readOnly}
            workspace={workspace}
          />
        </>
      );
    case "check-ins":
      return <AgentCheckInsEditor {...props} />;
    case "memory":
      return <AgentMemoryEditor {...props} />;
    case "where":
      return (
        <AgentWhereItWorks
          actions={profile.placeActions}
          capacity={profile.capacity}
          places={profile.places}
        />
      );
  }
}

/** A focused page of the agent, with the crumb back to its home. */
export function AgentPageLocation({ name, page }: { name: string; page: AgentEditorPage }) {
  const detail = useAgentDetail(name);
  useAgentDetailTopbar(detail, name, PAGE_TITLES[page], { publish: !detail.agent });
  if (detail.agentLoading) return <AgentDetailLoading />;
  if (detail.agentError || !detail.agent) {
    return <AgentNotFound error={detail.agentError} name={name} onBack={detail.onBackToAgents} />;
  }
  return (
    <div className={AGENT_PAGE_SCROLL_CLASS}>
      <PageContent className="max-w-5xl">
        <PageBody agent={detail.agent} detail={detail} page={page} />
      </PageContent>
    </div>
  );
}
