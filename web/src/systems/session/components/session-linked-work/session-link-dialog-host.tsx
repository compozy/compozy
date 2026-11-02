/** The session window's one link dialog: every entry opens it through the dialog store. */
import { useSessionLinkDialogModel } from "../../hooks/use-session-link-dialog-model";
import { useSessionLinkDialogOpen } from "../../hooks/use-session-link-dialog";
import { SessionLinkDialog } from "./session-link-dialog";

interface SessionLinkDialogHostProps {
  /** Two windows can show one session, and every entry acts on the focused one. */
  focused: boolean;
  sessionId: string;
  workspaceId: string;
  workspaceName: string;
  profile: string;
}

export function SessionLinkDialogHost({ focused, ...props }: SessionLinkDialogHostProps) {
  const open = useSessionLinkDialogOpen(props.sessionId, focused);
  return open ? <OpenSessionLinkDialog {...props} /> : null;
}

function OpenSessionLinkDialog(props: Omit<SessionLinkDialogHostProps, "focused">) {
  return <SessionLinkDialog {...useSessionLinkDialogModel(props)} />;
}
