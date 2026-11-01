import { AlertCircle, Compass } from "@compozy/ui/icons";
import { useNavigate } from "@tanstack/react-router";

import {
  Button,
  cn,
  Empty,
  PAGE_CONTENT_GUTTER,
  PageContent,
  Skeleton,
  SkeletonRegion,
} from "@compozy/ui";

import {
  AgentProfileHome,
  agentFromOrphanInstance,
  useAgentInstances,
  useAgentMemorySummary,
  useAgentProfile,
  useAgentWindowScope,
  type AgentDetailSearch,
  type AgentPayload,
  type AgentProfilePanel,
} from "@/systems/agent";

import { AGENT_PAGE_SCROLL_CLASS } from "./agent-page-frame";
import { useAgentDetail, type UseAgentDetailResult } from "./use-agent-detail";
import { useAgentDetailTopbar } from "./use-agent-detail-topbar";

export function AgentNotFound({
  name,
  error,
  onBack,
}: {
  name: string;
  error: Error | null;
  onBack: () => void;
}) {
  return (
    <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-1 items-center justify-center py-8")}>
      <Empty
        action={
          <Button
            data-testid="agent-detail-back-agents"
            onClick={onBack}
            size="sm"
            type="button"
            variant="secondary"
          >
            <Compass aria-hidden="true" />
            Back to agents
          </Button>
        }
        data-testid="agent-detail-not-found"
        description={error?.message ?? `No agent named "${name}" was found in this project.`}
        icon={AlertCircle}
        title="Agent not found"
      />
    </div>
  );
}

/** The home while the agent loads: the header's shape over the page's two columns. */
export function AgentDetailLoading() {
  return (
    <div className={AGENT_PAGE_SCROLL_CLASS} data-testid="agent-detail-loading">
      <PageContent className="max-w-6xl">
        <SkeletonRegion className="flex flex-col gap-6" label="Loading agent">
          <div className="flex items-center gap-5">
            <Skeleton className="size-20 rounded-full" />
            <div className="flex flex-1 flex-col gap-2">
              <Skeleton className="h-6 w-48" />
              <Skeleton className="h-4 w-80" />
            </div>
          </div>
          <Skeleton className="h-64 rounded-lg" />
        </SkeletonRegion>
      </PageContent>
    </div>
  );
}

function Home({
  agent,
  page,
  panel,
  setPanel,
}: {
  agent: AgentPayload;
  page: UseAgentDetailResult;
  panel: AgentProfilePanel | null;
  setPanel: (panel: AgentProfilePanel | null) => void;
}) {
  const profile = useAgentProfile({
    agent,
    panel,
    setPanel,
    active: page.active,
    memorySummary: useAgentMemorySummary(agent),
  });
  return (
    <div className={AGENT_PAGE_SCROLL_CLASS}>
      <PageContent className="max-w-6xl">
        {page.deleteDialog}
        <AgentProfileHome page={profile} />
      </PageContent>
    </div>
  );
}

/** The agent's home, with the old `section` and `tab` links landing on their panels. */
export function AgentDetailLocation({ name, search }: { name: string; search: AgentDetailSearch }) {
  const page = useAgentDetail(name);
  const navigate = useNavigate();
  const { bindingSearch } = useAgentWindowScope();
  const orphan = useAgentInstances().find(instance => instance.agent_name === name);
  useAgentDetailTopbar(page, name);

  if (page.agentLoading) return <AgentDetailLoading />;
  // A deleted definition keeps its instances; the home says why they can't resume.
  const agent = page.agent ?? (orphan ? agentFromOrphanInstance(orphan) : undefined);
  if (!agent) {
    return <AgentNotFound error={page.agentError} name={name} onBack={page.onBackToAgents} />;
  }
  return (
    <Home
      agent={agent}
      page={page}
      panel={search.panel ?? null}
      setPanel={panel =>
        void navigate({
          to: "/agents/$name",
          params: { name },
          search: { ...bindingSearch, ...(panel ? { panel } : {}) },
          replace: true,
        })
      }
    />
  );
}
