import { SessionPromptRuntimeContext } from "../contexts/session-prompt-runtime-context-value";
import { use, useEffect, useRef, useState } from "react";
import { useAui, useAuiState } from "@assistant-ui/react";
import { useSelector } from "@xstate/store-react";

import { sendFirstPrompt } from "../lib/session-first-prompt";
import { sessionStore } from "../stores/session-store";

interface UseSessionFirstPromptParams {
  sessionId: string;
  /** The same gate the Send control uses — a session that cannot prompt cannot receive one. */
  canPrompt: boolean;
}

/**
 * Delivers the message typed before this session existed.
 *
 * Session creation is a different surface from the composer, so the message
 * crosses between them through the session store rather than a navigation
 * parameter. Sending is a synchronization with the assistant-ui runtime — an
 * external system — which is why it belongs in an effect; exactly-once is
 * enforced by {@link sendFirstPrompt}, not by this hook.
 */
export function useSessionFirstPrompt({ sessionId, canPrompt }: UseSessionFirstPromptParams): void {
  const aui = useAui();
  const runtimeStore = use(SessionPromptRuntimeContext);
  const lifetime = useRef<AbortController | null>(null);
  const [staged, setStaged] = useState<{
    text: string;
    files: File[];
    done: () => void;
    fail: () => void;
    signal: AbortSignal;
  } | null>(null);
  const submitted = useRef<typeof staged>(null);
  const composer = useAuiState(state => state.composer);
  // The runtime refuses messages while it is disabled or still loading history,
  // and refuses them silently. Waiting for it is what keeps the claim honest:
  // nothing is consumed until there is something that can accept it.
  const threadAccepts = useAuiState(state => !state.thread.isDisabled && !state.thread.isLoading);
  const prompt = useSelector(sessionStore, snapshot => snapshot.context.firstPrompts[sessionId]);

  useEffect(() => {
    const controller = new AbortController();
    lifetime.current = controller;
    return () => controller.abort();
  }, [sessionId]);

  useEffect(() => {
    const signal = lifetime.current?.signal;
    if (!signal || !prompt || prompt.claimed || !canPrompt || !threadAccepts) {
      return;
    }
    // Text can append directly; files use the native composer upload lifecycle.
    sendFirstPrompt(
      sessionId,
      (text, first) => {
        if (first.runtime && runtimeStore) {
          runtimeStore.trigger.runtimeSelected({
            value: {
              provider: first.runtime.provider,
              model: first.runtime.model ?? "",
              reasoning_effort: first.runtime.reasoning_effort ?? "",
              ...(first.runtime.acp_options ? { acp_options: first.runtime.acp_options } : {}),
            },
          });
          runtimeStore.trigger.speedSelected({ speed: first.runtime.speed ?? "normal" });
        }
        if (!first.files?.length) {
          aui.thread.append(text);
          return;
        }
        return stageFilesAndSend(text, first.files);
      },
      signal
    );

    /** Uploads through native composer ownership and waits for its published state before dispatch. */
    async function stageFilesAndSend(text: string, files: File[]) {
      // Let the session's native attachment adapter own uploads and recovery.
      sessionStore.trigger.composerDraftChanged({ sessionId, text });
      aui.composer.setText(text);
      await Promise.all(
        files.map(file => {
          if (aui.composer.getState().attachments.some(attachment => attachment.file === file))
            return;
          return aui.composer.addAttachment(file);
        })
      );
      if (!signal || signal.aborted) return;
      await new Promise<void>((resolve, reject) => {
        const done = () => {
          signal.removeEventListener("abort", done);
          resolve();
        };
        signal.addEventListener("abort", done, { once: true });
        const fail = () => {
          signal.removeEventListener("abort", done);
          reject(new Error("First-message attachment upload failed"));
        };
        setStaged({ text, files, done, fail, signal });
      });
    }
  }, [aui, canPrompt, prompt, runtimeStore, sessionId, threadAccepts]);

  useEffect(() => {
    if (
      !staged ||
      staged.signal.aborted ||
      submitted.current === staged ||
      !canPrompt ||
      !threadAccepts
    )
      return;
    // addAttachment resolves before the React client publishes its next state.
    if (composer.attachments.length < staged.files.length) return;
    if (composer.attachments.some(attachment => attachment.status.type === "running")) return;
    // Claim before invoking the external runtime: repeated effect passes cannot send twice.
    submitted.current = staged;
    if (composer.attachments.some(attachment => attachment.status.type !== "requires-action")) {
      staged.fail();
      return;
    }
    // Release the handoff to the editable composer once upload succeeded.
    staged.done();
    // Keep edits made during upload in the composer for the operator to send.
    if (
      composer.text !== staged.text ||
      composer.attachments.length !== staged.files.length ||
      staged.files.some(file => !composer.attachments.some(attachment => attachment.file === file))
    )
      return;
    aui.composer.send();
  }, [aui, canPrompt, composer, staged, threadAccepts]);
}
