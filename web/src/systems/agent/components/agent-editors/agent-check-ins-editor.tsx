import { useAgentHeartbeat } from "../../hooks/use-agent-heartbeat";
import { useCheckInsEditor } from "../../hooks/use-check-ins-editor";
import { useUnsavedGuard } from "../../hooks/use-unsaved-guard";
import type { AgentHeartbeatPayload } from "../../types";
import type { AgentEditorProps } from "./agent-what-it-does-editor";
import { AgentCheckInsView } from "./check-ins-view";
import { EditorLoadError, EditorLoading } from "./editor-states";

function LoadedEditor({
  payload,
  ...props
}: AgentEditorProps & { payload: AgentHeartbeatPayload }) {
  const model = useCheckInsEditor(props.agent, props.workspace, payload, props.readOnly);
  const guard = useUnsavedGuard({ dirty: model.document.changes > 0, entityName: "Check-ins" });
  return (
    <>
      <AgentCheckInsView
        agent={props.agent}
        model={model}
        onAddToProject={props.onAddToProject}
        state={props.state}
      />
      {guard.confirmDialog}
    </>
  );
}

export function AgentCheckInsEditor(props: AgentEditorProps) {
  const heartbeat = useAgentHeartbeat(props.agent.name, props.workspace);
  if (heartbeat.data) return <LoadedEditor {...props} payload={heartbeat.data} />;
  if (heartbeat.isError) {
    return (
      <EditorLoadError onRetry={() => void heartbeat.refetch()} title="Couldn't load this page" />
    );
  }
  return <EditorLoading label="Loading check-ins" />;
}
