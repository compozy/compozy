/** The session window's one link dialog: every entry opens it through the dialog store. */
import { useSessionLinkDialogModel } from "../../hooks/use-session-link-dialog-model";
import { useSessionLinkDialogOpen } from "../../hooks/use-session-link-dialog";
import { SessionLinkDialog } from "./session-link-dialog";

interface SessionLinkDialogHostProps {
  sessionId: string;
  workspaceId: string;
  workspaceName: string;
  profile: string;
}

export function SessionLinkDialogHost(props: SessionLinkDialogHostProps) {
  return useSessionLinkDialogOpen(props.sessionId) ? <OpenSessionLinkDialog {...props} /> : null;
}

function OpenSessionLinkDialog(props: SessionLinkDialogHostProps) {
  return <SessionLinkDialog {...useSessionLinkDialogModel(props)} />;
}
