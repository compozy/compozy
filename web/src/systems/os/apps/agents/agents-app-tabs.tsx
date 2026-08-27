import { useNavigate } from "@tanstack/react-router";

import { LaneTabs, type LaneTabsItem } from "@compozy/ui";

import { agentsAppTabPath, type AgentsAppTab } from "./agent-window-location";

const TAB_ITEMS: ReadonlyArray<LaneTabsItem<AgentsAppTab>> = [
  { value: "catalog", label: "Catalog", testId: "agents-tab-catalog" },
  { value: "activity", label: "Activity", testId: "agents-tab-activity" },
];

export function AgentsAppTabs({ value }: { value: AgentsAppTab }) {
  const navigate = useNavigate({ from: "/agents" });

  return (
    <LaneTabs
      ariaLabel="Agents app locations"
      data-testid="agents-app-tabs"
      items={TAB_ITEMS}
      listClassName="w-full"
      onChange={next => {
        void navigate({ to: agentsAppTabPath(next) });
      }}
      value={value}
    />
  );
}
