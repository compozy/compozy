import { AlertCircle } from "@compozy/ui/icons";

import { AgentAvatar, Empty, PillGroup, SearchInput, useTopbarSlot } from "@compozy/ui";

import { agentAvatarAppearance, useAgentInstances } from "@/systems/agent";
import { PROFILE_SPRITE_URL } from "@/systems/profiles";

import { useActivityPage } from "../../hooks/use-activity-page";
import type { ActivityFilter } from "../../lib/activity-view";
import { ActivityPageView } from "./activity-page-view";
import { HomeDashboardSkeleton } from "../home-dashboard-skeleton";

const FILTERS: { value: ActivityFilter; label: string }[] = [
  { value: "all", label: "Everything" },
  { value: "unattended", label: "Nobody watching" },
];

export function ActivityPage({ onBack }: { onBack: () => void }) {
  const page = useActivityPage();
  const instances = useAgentInstances();

  useTopbarSlot({
    crumb: "Activity",
    crumbs: [{ id: "dashboard", label: "Dashboard", onSelect: onBack }],
    onBack,
    toolbar: (
      <>
        <PillGroup items={FILTERS} onChange={page.setFilter} size="sm" value={page.filter} />
        <SearchInput
          aria-label="Search activity"
          containerClassName="ml-auto w-64"
          onChange={page.setQuery}
          placeholder="Search activity…"
          value={page.query}
        />
      </>
    ),
  });

  const agentMark = (name: string, size: number) => {
    const instance = instances.find(candidate => candidate.agent_name === name);
    const label = instance?.label ?? name;
    return {
      label,
      avatar: (
        <AgentAvatar
          appearance={agentAvatarAppearance(instance?.resolved_appearance, name)}
          label={label}
          quietIdle
          size={size}
          spriteUrl={PROFILE_SPRITE_URL}
          surfaceClassName="ring-card"
        />
      ),
    };
  };

  if (page.error) {
    return (
      <Empty
        cause={page.error.message}
        className="min-h-full"
        icon={AlertCircle}
        role="alert"
        size="compact"
        title="Couldn't load activity"
      />
    );
  }
  if (!page.model) return <HomeDashboardSkeleton usageWindow={7} />;
  return (
    <ActivityPageView
      agentMark={agentMark}
      filter={page.filter}
      model={page.model}
      rows={page.rows}
      truncated={page.truncated}
      workspaceName={null}
    />
  );
}
