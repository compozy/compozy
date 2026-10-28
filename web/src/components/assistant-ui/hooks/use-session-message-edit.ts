import { useAui } from "@assistant-ui/react";
import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef } from "react";
import { toast } from "sonner";

import { createClientId } from "@/lib/client-id";
import { releaseComposerDraftFile } from "@/systems/session/lib/session-composer-attachment-drafts";
import type { UIMessage } from "ai";
import {
  attachmentsFromPromptMessageParts,
  sessionStore,
  discardSessionTerminalQuote,
} from "@/systems/session";
import { useSessionRewind } from "@/systems/session/hooks/use-session-rewind";
import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";
import type { SessionMessageEdit } from "../session-message-edit-context";
import type { SessionComposerState } from "./use-session-composer-state";

const editLogic = createStoreLogic({
  context: { messageId: null as string | null, idempotencyKey: "", pending: false },
  on: {
    selected: (context, event: { messageId: string; idempotencyKey: string }) =>
      context.pending
        ? undefined
        : { ...context, messageId: event.messageId, idempotencyKey: event.idempotencyKey },
    canceled: context => (context.pending ? undefined : { ...context, messageId: null }),
    submitting: context => (context.pending ? undefined : { ...context, pending: true }),
    settled: (context, event: { accepted: boolean }) => ({
      ...context,
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
      store.trigger.selected({ messageId, idempotencyKey: createClientId() });
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
      if (
        !current.messageId ||
        current.pending ||
        !runtime?.resetRuntime ||
        !runtime.sendDetachedPrompt
      )
        return;
      const submitted = aui.composer.getState();
      if (!submitted.text.trim()) return;
      const message: UIMessage = {
        id: createClientId(),
        role: "user",
        parts: [
          { type: "text", text: submitted.text },
          ...submitted.attachments.flatMap(attachment =>
            attachmentsFromPromptMessageParts(attachment.content).map(data => ({
              type: "data-compozy-attachment" as const,
              data,
            }))
          ),
        ],
      };
      store.trigger.submitting();
      try {
        await rewind.mutateAsync({
          idempotencyKey: current.idempotencyKey,
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
      if (!mounted.current) {
        try {
          await runtime.sendDetachedPrompt(message);
          for (const attachment of submitted.attachments) {
            if (attachment.file) releaseComposerDraftFile(attachment.file);
          }
          const draft = sessionStore.getSnapshot().context.drafts[runtime.sessionId];
          if (draft === submitted.text)
            sessionStore.trigger.composerDraftDiscarded({ sessionId: runtime.sessionId });
        } catch (error) {
          toast.error("Couldn't send the edited message.", {
            description: error instanceof Error ? error.message : undefined,
          });
        }
        return;
      }
      runtime.resetRuntime();
      // The normal send path owns attachments, prompt identity, and refused-send recovery.
      aui.composer.send();
    },
  };
}
