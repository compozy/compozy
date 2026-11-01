import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { AgentAvatar } from "@compozy/ui";

import { PROFILE_SPRITE_URL } from "@/systems/profiles/lib/profile-identity";

import { deriveActivityModel, type ActivityFilter } from "../../lib/activity-view";
import {
  ACTIVITY_NOW,
  activityEntriesFixture,
  activityRollupFixture,
} from "../../mocks/activity-fixtures";
import { ActivityPageView } from "./activity-page-view";

export const meta = definePrototype({
  title: "Activity page",
  spec: "always-on-agent",
  surface: "S14",
  route: "/activity",
  states: [
    { id: "audit", label: "Nobody watching" },
    { id: "audit-all", label: "Everything, with an unknown initiator" },
    { id: "audit-empty", label: "Nothing ran on its own" },
  ],
});

function agentMark(name: string, size: number) {
  return {
    label: name,
    avatar: (
      <AgentAvatar
        appearance={{ avatar: "generated", seed: name }}
        label={name}
        quietIdle
        size={size}
        spriteUrl={PROFILE_SPRITE_URL}
      />
    ),
  };
}

export default function ActivityPageProto({ state }: PrototypeProps) {
  const filter: ActivityFilter = state === "audit-all" ? "all" : "unattended";
  const empty = state === "audit-empty";
  const rollup = activityRollupFixture(filter === "unattended");
  const now = new Date(ACTIVITY_NOW);
  const model = deriveActivityModel(
    filter,
    empty ? [] : rollup.rollups,
    empty ? [] : activityEntriesFixture(filter === "unattended"),
    new Date(rollup.until),
    now
  );
  return (
    <ActivityPageView
      agentMark={agentMark}
      filter={filter}
      model={model}
      rows={model.rows}
      truncated={false}
      workspaceName="compozy"
    />
  );
}
