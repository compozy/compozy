import { useSessionParentPickerRequest } from "../hooks/use-session-parent-picker";
import { useSessionParentPickerModel } from "../hooks/use-session-parent-picker-model";
import { useSessionReparent } from "../hooks/use-session-reparent";
import { SessionParentPicker } from "./session-parent-picker";

type SessionReparent = ReturnType<typeof useSessionReparent>;

/**
 * The shell's one Move under… picker. The mutation lives here, outside the
 * per-request body, so its toast and Undo still land after the picker closes.
 */
export function SessionParentPickerHost() {
  const sessionId = useSessionParentPickerRequest();
  const reparent = useSessionReparent();
  if (sessionId === null) return null;
  return <OpenSessionParentPicker key={sessionId} reparent={reparent} sessionId={sessionId} />;
}

function OpenSessionParentPicker({
  sessionId,
  reparent,
}: {
  sessionId: string;
  reparent: SessionReparent;
}) {
  const model = useSessionParentPickerModel(sessionId, reparent);
  return <SessionParentPicker open {...model} />;
}
