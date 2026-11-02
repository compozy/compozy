import { useActiveWorkspace } from "@/systems/workspace";

import { useAgentMemory } from "../../hooks/use-agent-memory";
import { useMemoryEditor } from "../../hooks/use-memory-editor";
import { useUnsavedGuard } from "../../hooks/use-unsaved-guard";
import { agentScopeLabel } from "../../lib/agent-instance";
import type { AgentMemoryDocument } from "../../types";
import type { AgentEditorProps } from "./agent-what-it-does-editor";
import { EditorLoadError, EditorLoading } from "./editor-states";
import { AgentMemoryView } from "./memory-view";

function LoadedEditor({ payload, ...props }: AgentEditorProps & { payload: AgentMemoryDocument }) {
  const model = useMemoryEditor(props.agent, props.workspace, payload);
  const guard = useUnsavedGuard({ dirty: model.document.changes > 0, entityName: "Memory" });
  const { workspaces } = useActiveWorkspace();
  const scopeLabel = (scope: string) =>
    workspaces.find(item => item.id === scope)?.name ?? agentScopeLabel(scope);
  return (
    <>
      <AgentMemoryView
        agent={props.agent}
        model={model}
        scopeLabel={scopeLabel}
        state={props.state}
      />
      {guard.confirmDialog}
    </>
  );
}

/** Memory stays editable when an extension owns the definition: the notes are the operator's. */
export function AgentMemoryEditor(props: AgentEditorProps) {
  const memory = useAgentMemory(props.agent.name, props.workspace);
  if (memory.data) return <LoadedEditor {...props} payload={memory.data} />;
  if (memory.isError) {
    return (
      <EditorLoadError onRetry={() => void memory.refetch()} title="Couldn't load this page" />
    );
  }
  return <EditorLoading label="Loading memory" />;
}
