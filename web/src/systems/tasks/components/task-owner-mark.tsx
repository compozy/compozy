import type { AgentIdentityScope } from "@/systems/agent";
import { OwnerAvatar } from "@compozy/ui";

import { AgentIdentityMark } from "@/systems/agent/components/agent-identity-mark";

import { useTaskOwnerName, type TaskOwner } from "../hooks/use-task-owner-name";
import { ownerAvatarKindFor } from "../lib/task-formatters";

export function TaskOwnerName({
  owner,
  agentScope,
}: {
  owner?: TaskOwner | null;
  agentScope?: AgentIdentityScope;
}) {
  const name = useTaskOwnerName(owner, agentScope);
  return <>{name}</>;
}

/** Agents wear their own face; `OwnerAvatar` stays for people and the runtime's actors. */
export function TaskOwnerMark({
  owner,
  agentScope,
  "data-testid": testId,
}: {
  owner?: TaskOwner | null;
  agentScope?: AgentIdentityScope;
  "data-testid"?: string;
}) {
  const name = useTaskOwnerName(owner, agentScope);
  if (owner?.kind === "agent_session" && owner.ref) {
    return (
      <AgentIdentityMark
        agentScope={agentScope}
        data-testid={testId}
        name={owner.ref}
        quietIdle
        size={20}
      />
    );
  }
  return (
    <OwnerAvatar
      data-testid={testId}
      name={name}
      ownerId={owner?.ref ?? owner?.kind ?? "unassigned"}
      ownerKind={ownerAvatarKindFor(owner?.kind)}
      size="sm"
    />
  );
}
