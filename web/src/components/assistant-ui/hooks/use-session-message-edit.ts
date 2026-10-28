import { useAui } from "@assistant-ui/react";
import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef } from "react";
import { toast } from "sonner";

import { createClientId } from "@/lib/client-id";
import { discardSessionTerminalQuote } from "@/systems/session";
import { useSessionRewind } from "@/systems/session/hooks/use-session-rewind";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import type { SessionMessageEdit } from "../session-message-edit-context";
import type { SessionComposerState } from "./use-session-composer-state";

const editLogic = createStoreLogic({
  context: { messageId: null as string | null, pending: false },
  on: {
    selected: (context, event: { messageId: string }) =>
      context.pending ? undefined : { ...context, messageId: event.messageId },
    canceled: context => (context.pending ? undefined : { ...context, messageId: null }),
    submitting: context => (context.pending ? undefined : { ...context, pending: true }),
    settled: (context, event: { accepted: boolean }) => ({
      messageId: event.accepted ? null : context.messageId,
      pending: false,
    }),
  },
});

/** The edit belongs to the composer, which survives removal of the selected transcript row. */
export function useSessionMessageEdit(composer: SessionComposerState): SessionMessageEdit {
  const aui = useAui();
  const runtime = useSessionRuntimeRenderContext();
  const rewind = useSessionRewind(runtime?.workspaceId ?? "");
  const store = useStore(editLogic);
  const state = useSelector(store, snapshot => snapshot.context);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  useEffect(() => {
    if (composer.composerText.length === 0) store.trigger.canceled();
  }, [composer.composerText, store]);

  return {
    active: state.messageId !== null,
    pending: state.pending,
    start: (messageId, text) => {
      if (store.getSnapshot().context.pending) return;
      store.trigger.selected({ messageId });
      void aui.composer.clearAttachments();
      discardSessionTerminalQuote(runtime?.sessionId ?? "");
      composer.replaceComposer(text);
    },
    cancel: () => {
      if (store.getSnapshot().context.pending) return;
      store.trigger.canceled();
      composer.clearComposer();
    },
    submit: async () => {
      const current = store.getSnapshot().context;
      if (!current.messageId || current.pending || !runtime?.resetRuntime) return;
      if (!aui.composer.getState().text.trim()) return;
      store.trigger.submitting();
      try {
        await rewind.mutateAsync({
          idempotencyKey: createClientId(),
          messageId: current.messageId,
          sessionId: runtime.sessionId,
          signal: new AbortController().signal,
        });
      } catch (error) {
        store.trigger.settled({ accepted: false });
        toast.error("Couldn't edit this message.", {
          description:
            error instanceof Error ? error.message : "Refresh the conversation and try again.",
        });
        return;
      }
      store.trigger.settled({ accepted: true });
      if (!mounted.current) return;
      runtime.resetRuntime();
      // The normal send path owns attachments, prompt identity, and refused-send recovery.
      aui.composer.send();
    },
  };
}
