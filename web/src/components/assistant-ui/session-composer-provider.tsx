import type { ReactNode } from "react";

import type { SessionComposerProps } from "./session-composer";
import {
  SessionComposerActionsContext,
  SessionComposerMetaContext,
  SessionComposerStateContext,
} from "./hooks/use-session-composer-context";
import { useSessionComposerController } from "./hooks/use-session-composer-controller";
import type { SessionComposerState } from "./hooks/use-session-composer-state";

export function SessionComposerProvider({
  children,
  ...props
}: SessionComposerProps & { children: ReactNode; composerState: SessionComposerState }) {
  const controller = useSessionComposerController(props);
  return (
    <SessionComposerStateContext value={controller.state}>
      <SessionComposerActionsContext value={controller.actions}>
        <SessionComposerMetaContext value={controller.meta}>{children}</SessionComposerMetaContext>
      </SessionComposerActionsContext>
    </SessionComposerStateContext>
  );
}
