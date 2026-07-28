import { storyWorkspaceIds } from "@/storybook/fintech-scenario";
import { StorybookWorkspaceSetup } from "@/storybook/route-story-meta";

export function AgentWorkspaceSetup() {
  return <StorybookWorkspaceSetup workspaceId={storyWorkspaceIds.risk} />;
}
