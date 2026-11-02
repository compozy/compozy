import type { AgentAvatarState } from "@compozy/ui";

import { useAgentSoul } from "../../hooks/use-agent-soul";
import { useUnsavedGuard } from "../../hooks/use-unsaved-guard";
import { useWhatItDoesEditor } from "../../hooks/use-what-it-does-editor";
import type { AgentPayload, AgentSoulPayload } from "../../types";
import { EditorLoadError, EditorLoading } from "./editor-states";
import { AgentWhatItDoesView } from "./what-it-does-view";

export interface AgentEditorProps {
  agent: AgentPayload;
  /** The selector agent reads send: a project id, `@user`, or `null`. */
  workspace: string | null;
  state: AgentAvatarState;
  /** An extension owns the definition. */
  readOnly: boolean;
  onAddToProject?: () => void;
}

function LoadedEditor({ payload, ...props }: AgentEditorProps & { payload: AgentSoulPayload }) {
  const document = useWhatItDoesEditor(props.agent, props.workspace, payload);
  const guard = useUnsavedGuard({ dirty: document.changes > 0, entityName: "What it does" });
  return (
    <>
      <AgentWhatItDoesView
        agent={props.agent}
        document={document}
        readOnly={props.readOnly}
        state={props.state}
      />
      {guard.confirmDialog}
    </>
  );
}

export function AgentWhatItDoesEditor(props: AgentEditorProps) {
  const soul = useAgentSoul(props.agent.name, props.workspace);
  if (soul.data) return <LoadedEditor {...props} payload={soul.data} />;
  if (soul.isError) {
    return <EditorLoadError onRetry={() => void soul.refetch()} title="Couldn't load this page" />;
  }
  return <EditorLoading label="Loading what it does" />;
}
